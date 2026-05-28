package ordin

import (
	"html/template"
	"os"
	"time"

	"github.com/savuerka/ordin/framework"
)

type App = framework.App
type Router = framework.Router
type Context = framework.Context
type DB = framework.DB
type QueryBuilder = framework.QueryBuilder
type Migrator = framework.Migrator
type HandlerFunc = framework.HandlerFunc
type Middleware = framework.Middleware
type Option = framework.Option
type Resource = framework.Resource
type Data = framework.Data
type Renderer = framework.Renderer
type ViewEngine = framework.ViewEngine
type Storage = framework.Storage
type S3Storage = framework.S3Storage
type S3Config = framework.S3Config
type PutOptions = framework.PutOptions
type PutOption = framework.PutOption
type Queue = framework.Queue
type RabbitQueue = framework.RabbitQueue
type RabbitMQConfig = framework.RabbitMQConfig
type Job = framework.Job
type JobHandler = framework.JobHandler
type PublishOptions = framework.PublishOptions
type PublishOption = framework.PublishOption
type ConsumeOptions = framework.ConsumeOptions
type ConsumeOption = framework.ConsumeOption
type Cache = framework.Cache
type RedisCache = framework.RedisCache
type RedisConfig = framework.RedisConfig
type FileTransport = framework.FileTransport
type SFTPClient = framework.SFTPClient
type SFTPConfig = framework.SFTPConfig
type SFTPUploadOptions = framework.SFTPUploadOptions
type SFTPUploadOption = framework.SFTPUploadOption
type SFTPUploadResult = framework.SFTPUploadResult
type Scheduler = framework.Scheduler
type ScheduledJob = framework.ScheduledJob
type ScheduledFunc = framework.ScheduledFunc
type Trigger = framework.Trigger
type ScheduleOptions = framework.ScheduleOptions
type ScheduleOption = framework.ScheduleOption
type Pipeline = framework.Pipeline
type PipelineStep = framework.PipelineStep
type PipelineFunc = framework.PipelineFunc
type PipelineContext = framework.PipelineContext
type PipelineEvent = framework.PipelineEvent
type PipelineStepOptions = framework.PipelineStepOptions
type PipelineStepOption = framework.PipelineStepOption

var ErrCacheMiss = framework.ErrCacheMiss

func New(options ...Option) *App {
	return framework.New(options...)
}

func NewRouter() *Router {
	return framework.NewRouter()
}

func WithMiddleware(middlewares ...Middleware) Option {
	return framework.WithMiddleware(middlewares...)
}

func WithRenderer(renderer Renderer) Option {
	return framework.WithRenderer(renderer)
}

func WithViews(dir string, funcs ...template.FuncMap) Option {
	return framework.WithViews(dir, funcs...)
}

func WithStorage(storage Storage) Option {
	return framework.WithStorage(storage)
}

func WithQueue(queue Queue) Option {
	return framework.WithQueue(queue)
}

func WithCache(cache Cache) Option {
	return framework.WithCache(cache)
}

func WithRedis(cache Cache) Option {
	return framework.WithRedis(cache)
}

func WithSFTP(transport FileTransport) Option {
	return framework.WithSFTP(transport)
}

func WithScheduler(scheduler *Scheduler) Option {
	return framework.WithScheduler(scheduler)
}

func NewViewEngine(dir string, funcs ...template.FuncMap) (*ViewEngine, error) {
	return framework.NewViewEngine(dir, funcs...)
}

func MustViewEngine(dir string, funcs ...template.FuncMap) *ViewEngine {
	return framework.MustViewEngine(dir, funcs...)
}

func S3ConfigFromEnv(prefix string) S3Config {
	return framework.S3ConfigFromEnv(prefix)
}

func NewS3Storage(config S3Config) (*S3Storage, error) {
	return framework.NewS3Storage(config)
}

func MustS3Storage(config S3Config) *S3Storage {
	return framework.MustS3Storage(config)
}

func WithContentType(contentType string) PutOption {
	return framework.WithContentType(contentType)
}

func WithCacheControl(value string) PutOption {
	return framework.WithCacheControl(value)
}

func WithObjectMetadata(metadata map[string]string) PutOption {
	return framework.WithObjectMetadata(metadata)
}

func RabbitMQConfigFromEnv(prefix string) RabbitMQConfig {
	return framework.RabbitMQConfigFromEnv(prefix)
}

func NewRabbitQueue(config RabbitMQConfig) (*RabbitQueue, error) {
	return framework.NewRabbitQueue(config)
}

func MustRabbitQueue(config RabbitMQConfig) *RabbitQueue {
	return framework.MustRabbitQueue(config)
}

func WithQueueContentType(contentType string) PublishOption {
	return framework.WithQueueContentType(contentType)
}

func WithQueueHeaders(headers map[string]any) PublishOption {
	return framework.WithQueueHeaders(headers)
}

func WithExchange(exchange, routingKey string) PublishOption {
	return framework.WithExchange(exchange, routingKey)
}

func WithTransientMessage() PublishOption {
	return framework.WithTransientMessage()
}

func WithQueueDelay(delay time.Duration) PublishOption {
	return framework.WithQueueDelay(delay)
}

func WithConsumerName(name string) ConsumeOption {
	return framework.WithConsumerName(name)
}

func WithPrefetch(count int) ConsumeOption {
	return framework.WithPrefetch(count)
}

func WithAutoAck() ConsumeOption {
	return framework.WithAutoAck()
}

func WithRequeueOnError() ConsumeOption {
	return framework.WithRequeueOnError()
}

func RedisConfigFromEnv(prefix string) RedisConfig {
	return framework.RedisConfigFromEnv(prefix)
}

func NewRedisCache(config RedisConfig) (*RedisCache, error) {
	return framework.NewRedisCache(config)
}

func MustRedisCache(config RedisConfig) *RedisCache {
	return framework.MustRedisCache(config)
}

func SFTPConfigFromEnv(prefix string) SFTPConfig {
	return framework.SFTPConfigFromEnv(prefix)
}

func NewSFTPClient(config SFTPConfig) (*SFTPClient, error) {
	return framework.NewSFTPClient(config)
}

func MustSFTPClient(config SFTPConfig) *SFTPClient {
	return framework.MustSFTPClient(config)
}

func WithSFTPMkdirAll() SFTPUploadOption {
	return framework.WithSFTPMkdirAll()
}

func WithoutSFTPChecksum() SFTPUploadOption {
	return framework.WithoutSFTPChecksum()
}

func WithSFTPMode(mode os.FileMode) SFTPUploadOption {
	return framework.WithSFTPMode(mode)
}

func NewScheduler() *Scheduler {
	return framework.NewScheduler()
}

func RunImmediately() ScheduleOption {
	return framework.RunImmediately()
}

func Singleton() ScheduleOption {
	return framework.Singleton()
}

func WithScheduleTimeout(timeout time.Duration) ScheduleOption {
	return framework.WithScheduleTimeout(timeout)
}

func WithScheduleErrorHandler(handler func(string, error)) ScheduleOption {
	return framework.WithScheduleErrorHandler(handler)
}

func NewPipeline(name string) *Pipeline {
	return framework.NewPipeline(name)
}

func WithStepTimeout(timeout time.Duration) PipelineStepOption {
	return framework.WithStepTimeout(timeout)
}

func WithStepRetries(retries int, delay time.Duration) PipelineStepOption {
	return framework.WithStepRetries(retries, delay)
}

func ContinueOnStepError() PipelineStepOption {
	return framework.ContinueOnStepError()
}

func Dev() Option {
	return framework.Dev()
}

func Logger() Middleware {
	return framework.Logger()
}

func Recover() Middleware {
	return framework.Recover()
}

func Text(text string) HandlerFunc {
	return framework.Text(text)
}

func JSON(data any) HandlerFunc {
	return framework.JSON(data)
}

func Bind[T any](c *Context) (T, error) {
	return framework.Bind[T](c)
}

func ConnectPostgres(dsn string) (*DB, error) {
	return framework.ConnectPostgres(dsn)
}

func MustPostgres(dsn string) *DB {
	return framework.MustPostgres(dsn)
}

func MustPostgresEnv(key, fallback string) *DB {
	return framework.MustPostgresEnv(key, fallback)
}

func NewMigrator(db *DB) *Migrator {
	return framework.NewMigrator(db)
}

func MustMigrate(db *DB, dir string) {
	framework.MustMigrate(db, dir)
}

func Query[T any](db *DB, table string) *framework.TypedQuery[T] {
	return framework.Query[T](db, table)
}

func Repo[T any](db *DB, table string) *framework.TypedQuery[T] {
	return framework.Repo[T](db, table)
}
