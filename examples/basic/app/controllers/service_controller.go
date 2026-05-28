package controllers

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/savuerka/ordin"
)

type ServiceController struct{}

func (ServiceController) Upload(c *ordin.Context) error {
	if c.Storage() == nil {
		return c.Error(503, "storage is not configured")
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return c.BadRequest(err.Error())
	}
	defer file.Close()

	filename := safeFilename(header.Filename)
	key := "uploads/" + time.Now().UTC().Format("20060102T150405Z") + "-" + filename
	contentType := header.Header.Get("Content-Type")

	if err := c.Storage().Put(c.Ctx(), key, file, header.Size, ordin.WithContentType(contentType)); err != nil {
		return err
	}

	url, err := c.Storage().URL(c.Ctx(), key, 15*time.Minute)
	if err != nil {
		return err
	}

	return c.Created(ordin.Data{
		"key": key,
		"url": url,
	})
}

func (ServiceController) QueueWelcome(c *ordin.Context) error {
	if c.Queue() == nil {
		return c.Error(503, "queue is not configured")
	}

	email := c.Query("email")
	if strings.TrimSpace(email) == "" {
		email = "user@example.com"
	}

	if err := c.Queue().PublishJSON(c.Ctx(), "emails", ordin.Data{
		"type":  "welcome",
		"email": email,
	}); err != nil {
		return err
	}

	return c.Created(ordin.Data{
		"queued": true,
		"queue":  "emails",
	})
}

func (ServiceController) CacheDemo(c *ordin.Context) error {
	if c.Cache() == nil {
		return c.Error(503, "cache is not configured")
	}

	key := c.Query("key")
	if strings.TrimSpace(key) == "" {
		key = "demo:redis"
	}
	value := c.Query("value")
	if strings.TrimSpace(value) == "" {
		value = time.Now().UTC().Format(time.RFC3339)
	}

	if err := c.Cache().Set(c.Ctx(), key, value, 5*time.Minute); err != nil {
		return err
	}
	stored, err := c.Cache().Get(c.Ctx(), key)
	if err != nil {
		return err
	}

	return c.OK(ordin.Data{
		"key":   key,
		"value": stored,
		"ttl":   "5m",
	})
}

func (ServiceController) SFTPUpload(c *ordin.Context) error {
	if c.SFTP() == nil {
		return c.Error(503, "sftp transport is not configured")
	}

	localPath, remotePath, cleanup, err := saveRequestFile(c, "sftp")
	if err != nil {
		return c.BadRequest(err.Error())
	}
	defer cleanup()

	result, err := c.SFTP().Upload(c.Ctx(), localPath, remotePath, ordin.WithSFTPMkdirAll())
	if err != nil {
		return err
	}

	return c.Created(result)
}

func (ServiceController) SchedulerJobs(c *ordin.Context) error {
	if c.Scheduler() == nil {
		return c.Error(503, "scheduler is not configured")
	}

	jobs := make([]ordin.Data, 0)
	for _, job := range c.Scheduler().Jobs() {
		item := ordin.Data{"name": job.Name}
		if err := job.LastError(); err != nil {
			item["last_error"] = err.Error()
		}
		jobs = append(jobs, item)
	}
	return c.OK(ordin.Data{"jobs": jobs})
}

func (ServiceController) PipelineShipment(c *ordin.Context) error {
	if c.SFTP() == nil {
		return c.Error(503, "sftp transport is not configured")
	}

	localPath, remotePath, cleanup, err := saveRequestFile(c, "pipeline")
	if err != nil {
		return c.BadRequest(err.Error())
	}
	defer cleanup()

	pipeline := ordin.NewPipeline("file-shipment").
		Use("prepare", func(pc *ordin.PipelineContext) error {
			pc.Set("local_path", localPath)
			pc.Set("remote_path", remotePath)
			return nil
		}).
		Use("upload-sftp", func(pc *ordin.PipelineContext) error {
			result, err := c.SFTP().Upload(pc, pc.String("local_path"), pc.String("remote_path"), ordin.WithSFTPMkdirAll())
			if err != nil {
				return err
			}
			pc.Set("upload", result)
			return nil
		}, ordin.WithStepRetries(2, time.Second), ordin.WithStepTimeout(30*time.Second)).
		Use("publish-event", func(pc *ordin.PipelineContext) error {
			if c.Queue() == nil {
				pc.Set("event_published", false)
				return nil
			}
			if err := c.Queue().PublishJSON(pc, "shipments", ordin.Data{
				"type":        "file.shipped",
				"remote_path": pc.String("remote_path"),
			}); err != nil {
				return err
			}
			pc.Set("event_published", true)
			return nil
		}, ordin.ContinueOnStepError())

	result, err := pipeline.Run(c.Ctx(), ordin.Data{})
	if err != nil {
		return err
	}

	return c.Created(ordin.Data{
		"pipeline": result.Pipeline,
		"data":     result.Data,
		"events":   serializePipelineEvents(result.Events),
	})
}

func saveRequestFile(c *ordin.Context, folder string) (localPath string, remotePath string, cleanup func(), err error) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return "", "", func() {}, err
	}
	defer file.Close()

	filename := safeFilename(header.Filename)
	tmp, err := os.CreateTemp("", "ordin-upload-*")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup = func() { _ = os.Remove(tmp.Name()) }

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", "", func() {}, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", "", func() {}, err
	}

	remotePath = "/upload/" + strings.Trim(folder, "/") + "/" + time.Now().UTC().Format("20060102T150405Z") + "-" + filename
	return tmp.Name(), remotePath, cleanup, nil
}

func serializePipelineEvents(events []ordin.PipelineEvent) []ordin.Data {
	result := make([]ordin.Data, 0, len(events))
	for _, event := range events {
		item := ordin.Data{
			"step":        event.Step,
			"attempt":     event.Attempt,
			"duration_ms": event.Duration.Milliseconds(),
		}
		if event.Error != nil {
			item["error"] = event.Error.Error()
		}
		result = append(result, item)
	}
	return result
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}
