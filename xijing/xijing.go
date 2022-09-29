package xijing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/shafreeck/smile-upload/unwrap"
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
		name = id + "-" + name
		file := unwrap.Err(os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644))
		unwrap.Err(file.Write(data))
		resp.Body.Close()
		file.Close()
		fmt.Println("保存为: ", name)
		names = append(names, name)
	}
	return names
}
