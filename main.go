package main

import (
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"log"
	"path"

	"github.com/shafreeck/cortana"
	"github.com/shafreeck/miao/saes"
	"github.com/shafreeck/miao/smile"
	"github.com/shafreeck/miao/xijing"
)

func smileDecodeCommand() {
	args := struct {
		Text string
	}{}
	cortana.Parse(&args)

	b, err := aes.NewCipher(saes.AESKey)
	if err != nil {
		log.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString(args.Text)
	if err != nil {
		log.Fatal("decode base62 failed: ", err)
	}

	text := saes.AESDecrypt(b, data)
	fmt.Println(string(text))

}

func xijingDownloadCommand() {
	args := struct {
		IDs []string `cortana:"id, -, -"`
	}{}
	cortana.Parse(&args)

	xi := xijing.New("3bb8bf1e84b863a2de3a24bc24652b31", "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJodHRwOlwvXC9hcGkuYWlwaWF4aS5jb21cL3dlY2hhdFwvbG9naW5RUmNvZGVcLzRiMmNkZjQ2YThiMDU0NTM2MzJiMjdjNWQ4YzYyMWY5IiwiaWF0IjoxNjYzNTk5MDQxLCJleHAiOjE3MjU4MDcwNDEsIm5iZiI6MTY2MzU5OTA0MSwianRpIjoiR01COW5FemN0cjlJcXI2SiIsInN1YiI6IjY5MTMyNzEiLCJwcnYiOiIzNmIzM2RmOWM0YTRiYjU4ZDVlNzBhYWY5Y2M3NDIyMjFmYTg2ZTNiIn0.wXv7RfHSVj-zHmu7pmVzVUx7om2U6PQM8JFGr0eARJ8")
	for _, id := range args.IDs {
		xi.Download(id)
	}
}

func smileSendSMSCommand() {
	args := struct {
		Phone string `cortana:"--phone, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	sm.SendSMSCode(args.Phone)
}

func smileLoginCommand() {
	args := struct {
		Phone   string `cortana:"--phone, -, -"`
		SMSCode string `cortana:"--smscode, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	sm.Login(args.Phone, args.SMSCode)
}

func smileUploadOSSCommand() {
	args := struct {
		Paths []string `cortana:"path,,-"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	for _, path := range args.Paths {
		sm.UploadOSS(path)
	}
}

func smileUploadCommand() {
	args := struct {
		Paths []string `cortana:"path,,-"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	fmt.Println("开始上传")
	for _, p := range args.Paths {
		url, size := sm.UploadOSS(p)
		sm.CreateSongShare(path.Base(p), "佚名", url, size)
		fmt.Println(url)
	}
	fmt.Println("上传完成")
}

func downloadAndUploadCommand() {
	args := struct {
		IDs []string `cortana:"id, -, -"`
	}{}
	cortana.Parse(&args)

	fmt.Println("从戏鲸下载 ", args.IDs)
	var names []string
	xi := xijing.New("3bb8bf1e84b863a2de3a24bc24652b31", "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJodHRwOlwvXC9hcGkuYWlwaWF4aS5jb21cL3dlY2hhdFwvbG9naW5RUmNvZGVcLzRiMmNkZjQ2YThiMDU0NTM2MzJiMjdjNWQ4YzYyMWY5IiwiaWF0IjoxNjYzNTk5MDQxLCJleHAiOjE3MjU4MDcwNDEsIm5iZiI6MTY2MzU5OTA0MSwianRpIjoiR01COW5FemN0cjlJcXI2SiIsInN1YiI6IjY5MTMyNzEiLCJwcnYiOiIzNmIzM2RmOWM0YTRiYjU4ZDVlNzBhYWY5Y2M3NDIyMjFmYTg2ZTNiIn0.wXv7RfHSVj-zHmu7pmVzVUx7om2U6PQM8JFGr0eARJ8")
	for _, id := range args.IDs {
		names = append(names, xi.Download(id)...)
	}

	fmt.Println("上传四喵")
	sm := smile.New()
	for _, name := range names {
		url, size := sm.UploadOSS(name)
		sm.CreateSongShare(path.Base(name), "佚名", url, size)
		fmt.Println("上传：", name)
	}
	fmt.Println("上传完成")
}

func main() {
	cortana.AddRootCommand(downloadAndUploadCommand)
	cortana.AddCommand("download", xijingDownloadCommand, "从戏鲸下载BGM")
	cortana.AddCommand("decode", smileDecodeCommand, "解码四喵密文")
	cortana.AddCommand("sms", smileSendSMSCommand, "发送短信验证码")
	cortana.AddCommand("login", smileLoginCommand, "登录四喵")
	cortana.AddCommand("uploadoss", smileUploadOSSCommand, "上传 MP3 文件到 OSS")
	cortana.AddCommand("upload", smileUploadCommand, "上传 MP3 文件到 OSS 并创建分享")
	cortana.Launch()
}
