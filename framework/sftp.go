package framework

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// FileTransport describes a remote file delivery backend.
type FileTransport interface {
	Upload(ctx context.Context, localPath, remotePath string, options ...SFTPUploadOption) (*SFTPUploadResult, error)
	UploadReader(ctx context.Context, remotePath string, body io.Reader, options ...SFTPUploadOption) (*SFTPUploadResult, error)
	Close() error
}

// SFTPConfig configures SSH/SFTP delivery.
type SFTPConfig struct {
	Host                  string
	Port                  int
	Username              string
	Password              string
	PrivateKey            string
	PrivateKeyPath        string
	PrivateKeyPassphrase  string
	KnownHostsPath        string
	InsecureIgnoreHostKey bool
	Timeout               time.Duration
}

// SFTPConfigFromEnv reads SFTP_* variables by default.
func SFTPConfigFromEnv(prefix string) SFTPConfig {
	prefix = strings.Trim(strings.ToUpper(prefix), "_")
	if prefix == "" {
		prefix = "SFTP"
	}
	port, _ := strconv.Atoi(getenv(prefix+"_PORT", "22"))
	if port == 0 {
		port = 22
	}
	return SFTPConfig{
		Host:                  getenv(prefix+"_HOST", "localhost"),
		Port:                  port,
		Username:              getenv(prefix+"_USERNAME", ""),
		Password:              getenv(prefix+"_PASSWORD", ""),
		PrivateKey:            getenv(prefix+"_PRIVATE_KEY", ""),
		PrivateKeyPath:        getenv(prefix+"_PRIVATE_KEY_PATH", ""),
		PrivateKeyPassphrase:  getenv(prefix+"_PRIVATE_KEY_PASSPHRASE", ""),
		KnownHostsPath:        getenv(prefix+"_KNOWN_HOSTS_PATH", ""),
		InsecureIgnoreHostKey: getEnvironmentBool(prefix+"_INSECURE_IGNORE_HOST_KEY", false),
		Timeout:               getenvDuration(prefix+"_TIMEOUT", 15*time.Second),
	}
}

// SFTPClient uploads files to remote hosts through SFTP.
type SFTPClient struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

// NewSFTPClient creates a new SFTP client.
func NewSFTPClient(config SFTPConfig) (*SFTPClient, error) {
	if strings.TrimSpace(config.Host) == "" {
		return nil, errors.New("sftp host is empty")
	}
	if strings.TrimSpace(config.Username) == "" {
		return nil, errors.New("sftp username is empty")
	}

	auth, err := sftpAuthMethods(config)
	if err != nil {
		return nil, err
	}
	if len(auth) == 0 {
		return nil, errors.New("sftp auth method is empty")
	}

	hostKeyCallback, err := sftpHostKeyCallback(config)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         config.Timeout,
	}

	addr := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, err
	}

	return &SFTPClient{ssh: sshClient, sftp: sftpClient}, nil
}

// MustSFTPClient creates an SFTP client or panics.
func MustSFTPClient(config SFTPConfig) *SFTPClient {
	client, err := NewSFTPClient(config)
	if err != nil {
		panic(err)
	}
	return client
}

// Close closes the SFTP and SSH clients.
func (c *SFTPClient) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.sftp != nil {
		err = c.sftp.Close()
	}
	if c.ssh != nil {
		if closeErr := c.ssh.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

// SFTPUploadOptions configures an SFTP upload.
type SFTPUploadOptions struct {
	MkdirAll       bool
	VerifyChecksum bool
	Mode           os.FileMode
	knownSize      int64
}

// SFTPUploadOption updates SFTPUploadOptions.
type SFTPUploadOption func(*SFTPUploadOptions)

// WithSFTPMkdirAll creates the remote parent directory before upload.
func WithSFTPMkdirAll() SFTPUploadOption {
	return func(options *SFTPUploadOptions) {
		options.MkdirAll = true
	}
}

// WithoutSFTPChecksum disables post-upload checksum verification.
func WithoutSFTPChecksum() SFTPUploadOption {
	return func(options *SFTPUploadOptions) {
		options.VerifyChecksum = false
	}
}

// WithSFTPMode sets the remote file mode after upload.
func WithSFTPMode(mode os.FileMode) SFTPUploadOption {
	return func(options *SFTPUploadOptions) {
		options.Mode = mode
	}
}

// SFTPUploadResult describes the completed upload and checksum verification.
type SFTPUploadResult struct {
	RemotePath     string
	Size           int64
	ChecksumSHA256 string
	Verified       bool
}

// Upload uploads a local file and verifies SHA-256 after upload by default.
func (c *SFTPClient) Upload(ctx context.Context, localPath, remotePath string, options ...SFTPUploadOption) (*SFTPUploadResult, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	options = append([]SFTPUploadOption{withSFTPKnownSize(stat.Size())}, options...)
	return c.UploadReader(ctx, remotePath, file, options...)
}

// UploadReader uploads content from a reader and verifies SHA-256 after upload by default.
func (c *SFTPClient) UploadReader(ctx context.Context, remotePath string, body io.Reader, options ...SFTPUploadOption) (*SFTPUploadResult, error) {
	if c == nil || c.sftp == nil {
		return nil, errors.New("sftp client is not configured")
	}
	if body == nil {
		return nil, errors.New("sftp upload body is nil")
	}
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "" {
		return nil, errors.New("sftp remote path is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	upload := SFTPUploadOptions{MkdirAll: true, VerifyChecksum: true, Mode: 0644, knownSize: -1}
	for _, option := range options {
		if option != nil {
			option(&upload)
		}
	}

	if upload.MkdirAll {
		if err := c.sftp.MkdirAll(path.Dir(remotePath)); err != nil {
			return nil, err
		}
	}

	remote, err := c.sftp.Create(remotePath)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	written, copyErr := io.Copy(remote, io.TeeReader(body, hash))
	closeErr := remote.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if upload.knownSize >= 0 && written != upload.knownSize {
		return nil, fmt.Errorf("sftp upload size mismatch: local=%d remote=%d", upload.knownSize, written)
	}
	if upload.Mode != 0 {
		if err := c.sftp.Chmod(remotePath, upload.Mode); err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	result := &SFTPUploadResult{RemotePath: remotePath, Size: written, ChecksumSHA256: checksum}
	if upload.VerifyChecksum {
		remoteChecksum, err := c.remoteSHA256(ctx, remotePath)
		if err != nil {
			return nil, err
		}
		if remoteChecksum != checksum {
			return nil, fmt.Errorf("sftp checksum mismatch: local=%s remote=%s", checksum, remoteChecksum)
		}
		result.Verified = true
	}
	return result, nil
}

func (c *SFTPClient) remoteSHA256(ctx context.Context, remotePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	remote, err := c.sftp.Open(remotePath)
	if err != nil {
		return "", err
	}
	defer remote.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, remote); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func withSFTPKnownSize(size int64) SFTPUploadOption {
	return func(options *SFTPUploadOptions) {
		options.knownSize = size
	}
}

func sftpAuthMethods(config SFTPConfig) ([]ssh.AuthMethod, error) {
	auth := make([]ssh.AuthMethod, 0, 2)
	if config.Password != "" {
		auth = append(auth, ssh.Password(config.Password))
	}

	key := strings.TrimSpace(config.PrivateKey)
	if key == "" && strings.TrimSpace(config.PrivateKeyPath) != "" {
		data, err := os.ReadFile(config.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		key = string(data)
	}
	if key != "" {
		var signer ssh.Signer
		var err error
		if config.PrivateKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(config.PrivateKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(key))
		}
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	return auth, nil
}

func sftpHostKeyCallback(config SFTPConfig) (ssh.HostKeyCallback, error) {
	if config.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	if strings.TrimSpace(config.KnownHostsPath) == "" {
		return nil, errors.New("sftp known_hosts path is required unless insecure host key mode is enabled")
	}
	return knownhosts.New(config.KnownHostsPath)
}

func cleanRemotePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return path.Clean("/" + strings.TrimLeft(value, "/"))
}
