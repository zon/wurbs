package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/zon/chat/core"
)

func getMessages(c *fiber.Ctx) error {
	var messages []core.Message

	bq := c.Query("before")
	if bq != "" {
		before, err := core.ParseTime(bq)
		if err != nil {
			return fiber.ErrBadRequest
		}
		err = core.GetMessagesBefore(before, &messages)
		if err != nil {
			return err
		}
		return c.JSON(messages)
	}

	aq := c.Query("after")
	if aq != "" {
		after, err := core.ParseTime(aq)
		if err != nil {
			return fiber.ErrBadRequest
		}
		err = core.GetMessagesAfter(after, &messages)
		if err != nil {
			return err
		}
		return c.JSON(messages)
	}

	err := core.GetLatestMessages(&messages)
	if err != nil {
		return err
	}
	return c.JSON(messages)
}

func postMessage(c *fiber.Ctx) error {
	user, err := core.AuthUser(c)
	if err != nil {
		return err
	}

	var body string
	err = c.BodyParser(&body)
	if err != nil {
		return err
	}
	content := strings.TrimSpace(body)

	if content == "" {
		return fiber.ErrBadRequest
	}

	content, err = core.MarkdownToHtml(content)
	if err != nil {
		return err
	}
	record, err := core.CreateMessage(user.ID, content)
	if err != nil {
		return err
	}

	err = core.PublishMessage("created", record)
	if err != nil {
		return err
	}

	return c.JSON(record)
}

func putMessage(c *fiber.Ctx) error {
	user, err := core.AuthUser(c)
	if err != nil {
		return err
	}

	id := c.Params("id")
	var msgID uint
	_, err = fmt.Sscanf(id, "%d", &msgID)
	if err != nil {
		return fiber.ErrBadRequest
	}

	var body string
	err = c.BodyParser(&body)
	if err != nil {
		return err
	}
	content := strings.TrimSpace(body)

	if content == "" {
		return fiber.ErrBadRequest
	}

	content, err = core.MarkdownToHtml(content)
	if err != nil {
		return err
	}

	msg := &core.Message{}
	msg.ID = msgID
	err = core.DB.First(msg).Error
	if err != nil {
		return fiber.ErrNotFound
	}

	if msg.UserID != user.ID {
		return fiber.ErrForbidden
	}

	err = msg.Update(content)
	if err != nil {
		return err
	}

	err = core.PublishMessage("updated", msg)
	if err != nil {
		return err
	}

	return c.JSON(msg)
}

func deleteMessage(c *fiber.Ctx) error {
	user, err := core.AuthUser(c)
	if err != nil {
		return err
	}

	id := c.Params("id")
	var msgID uint
	_, err = fmt.Sscanf(id, "%d", &msgID)
	if err != nil {
		return fiber.ErrBadRequest
	}

	msg := &core.Message{}
	msg.ID = msgID
	err = core.DB.First(msg).Error
	if err != nil {
		return fiber.ErrNotFound
	}

	if msg.UserID != user.ID {
		return fiber.ErrForbidden
	}

	err = msg.Delete()
	if err != nil {
		return err
	}

	err = core.PublishMessage("deleted", msg)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
