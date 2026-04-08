package main

import (
	"context"
	"fmt"
	"github.com/kittenbark/tg"
	"github.com/kittenbark/tg-twitter/vxtwitter"
	"github.com/kittenbark/tgmedia/tgvideo"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	vx := vxtwitter.New()
	tg.NewFromEnv().
		Scheduler().
		OnError(tg.OnErrorLog).
		Command("/start", tg.CommonReactionReply("💅")).
		Branch(tg.OnUrl, tg.Chain(tg.CommonReactionReply("👀"), tg.Synced(func(ctx context.Context, upd *tg.Update) error {
			msg := upd.Message
			reply := &tg.ReplyParameters{MessageId: msg.MessageId}
			url, err := vxtwitter.Vx(msg.Text)
			if err != nil {
				if _, err := tg.SendMessage(ctx, msg.Chat.Id, "url parsing failed", &tg.OptSendMessage{ReplyParameters: reply}); err != nil {
					return err
				}
			}

			slog.Info("downloading", "from", msg.Chat.Username, "twitter", msg.Text, "vx", url)
			files, dir, post, err := vx.DownloadTempVx(url)
			defer func(dir string) {
				if err := os.RemoveAll(dir); err != nil {
					slog.Error("failed to remove temporary files", "err", err)
				}
			}(dir)
			if err != nil {
				slog.Error("failed to download", "err", err)
			}

			pictures := make([]string, 0, len(files))
			for i, file := range files {
				filename := path.Join(dir, fmt.Sprintf(
					"%s_%s_%s",
					strings.ToLower(post.UserScreenName),
					time.Unix(post.DateEpoch, 0).Format("2006-01-02"),
					path.Base(file),
				))
				if err := os.Rename(file, filename); err != nil {
					slog.Error("failed to move file", "err", err)
					continue
				}
				switch path.Ext(strings.ToLower(filename)) {
				case ".mp4", ".mpeg", ".mov":
					if _, err := tgvideo.Send(ctx, msg.Chat.Id, filename, &tg.OptSendVideo{ReplyParameters: reply}); err != nil {
						return err
					}
				case ".png", ".jpg", ".jpeg":
					if i+1 == len(files) {
						if err := sendPhotoAsDocumentWithUploadButton(ctx, msg, filename, reply, pictures); err != nil {
							return err
						}
						break
					}
					sent, err := tg.SendDocument(ctx, msg.Chat.Id, tg.FromDisk(filename), &tg.OptSendDocument{ReplyParameters: reply})
					if err != nil {
						return err
					}
					pictures = append(pictures, sent.Document.FileId)
				default:
					if _, err := tg.SendDocument(ctx, msg.Chat.Id, tg.FromDisk(filename), &tg.OptSendDocument{ReplyParameters: reply}); err != nil {
						return err
					}
				}
			}
			return nil
		}))).
		Start()
}

var onUrlPicId = &atomic.Int64{}

func sendPhotoAsDocumentWithUploadButton(
	ctx context.Context,
	msg *tg.Message,
	filename string,
	reply *tg.ReplyParameters,
	picturesFileIds []string,
) error {
	sent, err := tg.SendDocument(
		ctx,
		msg.Chat.Id,
		tg.FromDisk(filename),
		&tg.OptSendDocument{
			ReplyParameters: reply,
			ReplyMarkup: (&tg.Keyboard{Layout: [][]tg.ButtonI{{
				&tg.CallbackButton{
					Text: fmt.Sprintf("upload pic #%d", onUrlPicId.Add(1)),
					Handler: func(ctx context.Context, upd *tg.Update) error {
						slog.Info(fmt.Sprintf("upload pic #%d", onUrlPicId.Add(1)))
						album := tg.Album{}
						for _, fileid := range picturesFileIds {
							file, err := tg.GetFile(ctx, fileid)
							if err != nil {
								return err
							}
							local, err := file.DownloadTemp(ctx)
							if err != nil {
								return err
							}
							defer func() { _ = os.Remove(local) }()
							album = append(album, &tg.Photo{Media: tg.FromDisk(local)})
						}

						if _, err := tg.SendMediaGroup(ctx, msg.Chat.Id, album, &tg.OptSendMediaGroup{ReplyParameters: reply}); err != nil {
							return err
						}
						return nil
					},
					OnComplete: &tg.OptAnswerCallbackQuery{
						Text:      "🍓",
						CacheTime: 30,
					},
				},
			}}}).BuildRegister(ctx),
		},
	)
	if err != nil {
		return err
	}
	picturesFileIds = append(picturesFileIds, sent.Document.FileId)
	return nil
}
