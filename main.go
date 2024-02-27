package main

import (
	"bufio"
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/shafreeck/cortana"
	"github.com/shafreeck/miao/saes"
	"github.com/shafreeck/miao/smile"
	"github.com/shafreeck/miao/unwrap"
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

func xijingSearchCommand() {
	args := struct {
		Keyword []string `cortana:"keyword, -, -"`
	}{}

	cortana.Parse(&args)
	xi := xijing.New("", "")

	var ids []int
	for _, term := range args.Keyword {
		ids = append(ids, xi.Search(term)...)
	}
	for _, id := range ids {
		fmt.Println(id)
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
		Paths  []string `cortana:"path,,-"`
		Author string   `cortana:"--author,,佚名"`
		Prefix string   `cortana:"--prefix"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	fmt.Println("开始上传")
	for _, p := range args.Paths {
		url, size := sm.UploadOSS(p)
		base := path.Base(p)
		ext := path.Ext(base)
		sm.CreateSongShare(args.Prefix+base[:len(base)-len(ext)], args.Author, url, size)
		fmt.Println("上传：", p, url)
	}
	fmt.Println("上传完成")
}

func downloadAndUploadCommand() {
	args := struct {
		Author string   `cortana:"--author,,佚名"`
		IDs    []string `cortana:"id, -, -"`
	}{}
	cortana.Parse(&args)

	idIndex := make(map[string][]string)
	files := unwrap.Err(os.ReadDir("bgm/"))
	for _, file := range files {
		name := file.Name()
		i := strings.Index(name, "-")
		if i < 0 {
			continue
		}
		if !strings.HasSuffix(name, ".mp3") {
			continue
		}

		id := name[:i]
		idIndex[id] = append(idIndex[id], "bgm/"+name)
	}

	var names []string
	xi := xijing.New("3bb8bf1e84b863a2de3a24bc24652b31", "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJodHRwOlwvXC9hcGkuYWlwaWF4aS5jb21cL3dlY2hhdFwvbG9naW5RUmNvZGVcLzRiMmNkZjQ2YThiMDU0NTM2MzJiMjdjNWQ4YzYyMWY5IiwiaWF0IjoxNjYzNTk5MDQxLCJleHAiOjE3MjU4MDcwNDEsIm5iZiI6MTY2MzU5OTA0MSwianRpIjoiR01COW5FemN0cjlJcXI2SiIsInN1YiI6IjY5MTMyNzEiLCJwcnYiOiIzNmIzM2RmOWM0YTRiYjU4ZDVlNzBhYWY5Y2M3NDIyMjFmYTg2ZTNiIn0.wXv7RfHSVj-zHmu7pmVzVUx7om2U6PQM8JFGr0eARJ8")
	for _, id := range args.IDs {
		if name, ok := idIndex[id]; ok {
			names = append(names, name...)
		} else {
			names = append(names, xi.Download(id)...)
		}
	}

	// convert to mp3 if necessary
	names = xijing.MP3Codec(names)

	fmt.Println("上传四喵")
	sm := smile.New()
	for _, name := range names {
		url, size := sm.UploadOSS(name)
		base := path.Base(name)
		ext := path.Ext(base)
		sm.CreateSongShare(base[:len(base)-len(ext)], args.Author, url, size)
		fmt.Println("上传：", name, url)
	}
	fmt.Println("上传完成")
}

func mp3codeCommand() {
	args := struct {
		Paths []string `cortana:"path,,-"`
	}{}
	cortana.Parse(&args)

	xijing.MP3Codec(args.Paths)
}

// the if the token is still valid
func miaoCommand() {
	sm := smile.New()
	songs := sm.ListSongs()
	for _, song := range songs {
		fmt.Printf("%-8d%s\n", song.ID, song.Name)
	}
	fmt.Println("Total: ", len(songs))
}

func segmentCommand() {
	args := struct {
		Paths    []string `cortana:"path,,-"`
		Duration string   `cortana:"--duration, -d, 600, duration in seconds"`
	}{}
	cortana.Parse(&args)

	for _, name := range args.Paths {
		st := unwrap.Err(os.Stat(name))
		if st.Size() < 20*1024*1024 {
			fmt.Println("忽略小于 20M 的文件：", name)
			continue
		}
		fmt.Println("切割 MP3: ", name)
		ext := path.Ext(name)
		bare := name[:len(name)-len(ext)]
		args := []string{"-y", "-hide_banner", "-loglevel", "error",
			"-i", name, "-c", "copy", "-f", "segment", "-segment_time", args.Duration,
			"-reset_timestamps", "1", bare + "-%0d.mp3"}

		fmt.Println("执行：", "ffmpeg ", args)
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		unwrap.Must(cmd.Run())
	}
	fmt.Println("切割完成")
}

func switchUserCommand() {
	args := struct {
		Phone string `cortana:"phone, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	sm.SendSMSCode(args.Phone)

	fmt.Printf("sms code:")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	smsCode := strings.TrimSpace(scanner.Text())
	sm.Login(args.Phone, smsCode)
}

func removeSongsCommand() {
	args := struct {
		IDs []int64 `cortana:"id"`
	}{}
	cortana.Parse(&args)

	sm := smile.New()
	sm.RemoveSongs(args.IDs...)
	fmt.Println("删除成功")
}

func main() {
	cortana.AddRootCommand(downloadAndUploadCommand)
	cortana.AddCommand("download", xijingDownloadCommand, "从戏鲸下载BGM")
	cortana.AddCommand("search", xijingSearchCommand, "从戏鲸搜索BGM")
	cortana.AddCommand("decode", smileDecodeCommand, "解码四喵密文")
	cortana.AddCommand("sms", smileSendSMSCommand, "发送短信验证码")
	cortana.AddCommand("login", smileLoginCommand, "登录四喵")
	cortana.AddCommand("oss upload", smileUploadOSSCommand, "上传 MP3 文件到 OSS")
	cortana.AddCommand("upload", smileUploadCommand, "上传 MP3 文件到 OSS 并创建分享")
	cortana.AddCommand("mp3codec", mp3codeCommand, "把音频文件转为 mp3 格式")
	cortana.AddCommand("miao", miaoCommand, "列出已上传的文件")
	cortana.AddCommand("segment", segmentCommand, "分割 MP3")
	cortana.AddCommand("switch", switchUserCommand, "切换用户")
	cortana.AddCommand("remove", removeSongsCommand, "删除歌曲")
	cortana.Launch()
}
