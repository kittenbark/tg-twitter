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

			files, dir, post, err := vx.DownloadTempVx(url)
			defer func(dir string) {
				if err := os.RemoveAll(dir); err != nil {
					slog.Error("failed to remove temporary files", "err", err)
				}
			}(dir)

			for _, file := range files {
				filename := fmt.Sprintf("@%s_%s", post.UserScreenName, path.Base(file))
				switch path.Ext(strings.ToLower(filename)) {
				case ".mp4", ".mpeg", ".mov":
					if _, err := tgvideo.Send(ctx, msg.Chat.Id, file, &tg.OptSendVideo{ReplyParameters: reply}); err != nil {
						return err
					}
				default:
					if _, err := tg.SendDocument(ctx, msg.Chat.Id, tg.FromDisk(file), &tg.OptSendDocument{ReplyParameters: reply}); err != nil {
						return err
					}
				}
			}
			return nil
		}))).
		Start()
}
