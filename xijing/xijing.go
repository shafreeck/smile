package xijing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/shafreeck/miao/unwrap"
)

type Client struct {
	authKey       string
	authorization string
}

func New(key, token string) *Client {
	return &Client{
		authKey:       key,
		authorization: "bearer " + token,
	}
}

func (x *Client) Login(phone string) {

}

type BGMURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type DownloadURLs struct {
	R    int      `json:"r"`
	Data []BGMURL `json:"data"`
}

func (c *Client) parseDownloadURLs(id string) DownloadURLs {
	url := fmt.Sprintf("https://api.aipiaxi.com/article/v1/%s/bgmlist?download=1", id)
	req := unwrap.Err(http.NewRequest(http.MethodGet, url, nil))
	req.Header.Add("Auth-Key", c.authKey)
	req.Header.Add("Authorization", c.authorization)
	resp := unwrap.Err(http.DefaultClient.Do(req))
	urls := DownloadURLs{}

	data := unwrap.Err(io.ReadAll(resp.Body))
	resp.Body.Close()

	unwrap.Err("", json.Unmarshal(data, &urls))
	return urls
}
func (c *Client) Download(id string) []string {
	urls := c.parseDownloadURLs(id)

	var names []string
	for _, d := range urls.Data {
		fmt.Println("解析到下載地址: ", d.URL)
		u := unwrap.Err(url.Parse(d.URL))
		name := u.Query().Get("attname")
		fmt.Println("下載: ", name)
		resp := unwrap.Err(http.Get(d.URL))
		data := unwrap.Err(io.ReadAll(resp.Body))

		// add the id as part of the name
		name = "bgm/" + id + "-" + name
		file := unwrap.Err(os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644))
		unwrap.Err(file.Write(data))
		resp.Body.Close()
		file.Close()
		fmt.Println("保存为: ", name)
		names = append(names, name)
	}
	return names
}

// translate to mp3 when necessary
func MP3Codec(names []string) []string {

	var targets []string
	for _, name := range names {
		ext := path.Ext(name)
		if ext == ".mp3" {
			targets = append(targets, name)
			continue
		}
		fmt.Println("转码 MP3: ", name)
		bare := strings.TrimRight(name, ext)
		args := []string{"-y", "-hide_banner", "-loglevel", "error",
			"-i", name, "-c:a", "libmp3lame", "-q:a", "8", bare + ".mp3"}

		fmt.Println("执行：", "ffmpeg ", args)
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		unwrap.Must(cmd.Run())
		targets = append(targets, bare+".mp3")
	}
	return targets
}
