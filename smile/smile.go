package smile

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/shafreeck/smile-upload/saes"
	"github.com/shafreeck/smile-upload/unwrap"
)

const Endpoint = "http://www.smilemiao.com"
const PlatformHeader = "pc_web"
const APIPath = "/api/v2"

type Client struct {
	endpoint string
	token    string
	aesb     cipher.Block
}

func New() *Client {
	var token string
	f, err := os.Open("smile-token.dat")
	if err == nil {
		token = strings.TrimSpace(string(unwrap.Err(io.ReadAll(f))))
	}
	b := unwrap.Err(aes.NewCipher(saes.AESKey))
	return &Client{endpoint: Endpoint, aesb: b, token: token}
}

func (c *Client) url(path string) string {
	return fmt.Sprintf("%s%s/%s", c.endpoint, APIPath, path)
}
func (c *Client) httpGet(path string) *http.Response {
	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodGet, url, nil))
	req.Header.Add("platform", PlatformHeader)
	req.Header.Add("token", c.token)
	return unwrap.Err(http.DefaultClient.Do(req))
}
func (c *Client) httpPost(path string, payload []byte) *http.Response {
	params := struct {
		Params string `json:"params"`
	}{Params: string(payload)}

	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodPost, url, bytes.NewReader(unwrap.Err(json.Marshal(params)))))
	req.Header.Add("platform", PlatformHeader)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("token", c.token)
	return unwrap.Err(http.DefaultClient.Do(req))

}
func (c *Client) SendSMSCode(phone string) {
	resp := c.httpGet("community/ucenter/sms?phone=" + phone + "&command=6")
	defer resp.Body.Close()

	data := unwrap.Err(io.ReadAll(resp.Body))
	plain := saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data))))
	fmt.Println("decode response: ", string(plain))

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{}

	unwrap.Must(json.Unmarshal(plain, &ok))
	if ok.Code != 200 {
		log.Fatal("get sms code failed, check your phone number please")
	}
}

func (c *Client) Login(phone, smscode string) {
	auth := struct {
		Field   string `json:"field"`
		SMSCode string `json:"smsCode"`
	}{
		Field:   phone,
		SMSCode: smscode,
	}
	payload := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(auth)))
	encoded := base64.StdEncoding.EncodeToString(payload)
	resp := c.httpPost("community/ucenter", []byte(encoded))
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	fmt.Println(string(data))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Fatal("login failed: ", ok.Message)
	}

	fmt.Println("get the auth token: ", ok.Data)
	f := unwrap.Err(os.OpenFile("smile-token.dat", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644))
	unwrap.Err(f.WriteString(ok.Data))
	c.token = ok.Data
}

// Upload the file and return the oss url
func (c *Client) UploadOSS(path string) (string, int64) {
	resp := c.httpGet("tpservice/aliyunsts/gettoken")
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			SecurityToken   string `json:"securityToken"`
			AccessKeySecret string `json:"accessKeySecret"`
			AccessKeyId     string `json:"accessKeyId"`
		} `json:"data"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))
	if ok.Code != 200 {
		log.Fatal("get oss token failed: ", ok.Message)
	}

	endpoint := "https://oss-cn-beijing.aliyuncs.com"
	ossCli := unwrap.Err(oss.New(endpoint, ok.Data.AccessKeyId, ok.Data.AccessKeySecret, oss.SecurityToken(ok.Data.SecurityToken)))
	b := unwrap.Err(ossCli.Bucket("smilemiao"))

	now := time.Now()
	key := fmt.Sprintf("music/%d.mp3", now.UnixMilli())
	fmt.Println("upload oss key: ", key)
	f := unwrap.Err(os.Open(path))
	finfo := unwrap.Err(f.Stat())
	unwrap.Must(b.PutObject(key, f))

	return "https://smilemiao.oss-cn-beijing.aliyuncs.com/" + key, finfo.Size() / 1024
}

func (c *Client) CreateSongShare(name, singer, url string, size int64) {
	params := struct {
		Name   string `json:"name"`
		Singer string `json:"singer"`
		Url    string `json:"url"`
		Type   int    `json:"type"`
		Size   int64  `json:"size"`
	}{
		Name:   name,
		Singer: singer,
		Url:    url,
		Type:   2,
		Size:   size,
	}

	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)

	resp := c.httpPost("/community/songshare/create", []byte(encoded))
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Fatal("create song share failed: ", ok.Message)
	}
}
