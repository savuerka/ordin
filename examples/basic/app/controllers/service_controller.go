package controllers

import (
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

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}
