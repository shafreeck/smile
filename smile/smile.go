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
	"github.com/shafreeck/miao/saes"
	"github.com/shafreeck/miao/unwrap"
)

const DefaultWebEndpoint = "http://www.smilemiao.com/api/v2"
const DefaultAPIEndpoint = "https://api.smilemiao.com/v2"
const PlatformHeader = "ios"

type Client struct {
	endpoint string
	token    string
	aesb     cipher.Block
}

type Option func(c *Client)

func Endpoint(ep string) Option {
	return func(c *Client) {
		c.endpoint = ep
	}
}

func New(opts ...Option) *Client {
	c := &Client{endpoint: DefaultWebEndpoint}

	for _, o := range opts {
		o(c)
	}

	// read token from file
	var token string
	f, err := os.Open("smile-token.dat")
	if err == nil {
		token = strings.TrimSpace(string(unwrap.Err(io.ReadAll(f))))
	}
	c.token = token

	// init the aes block
	b := unwrap.Err(aes.NewCipher(saes.AESKey))
	c.aesb = b

	return c
}

func NewWebClient() *Client {
	return New(Endpoint(DefaultWebEndpoint))
}

func NewAPIClient() *Client {
	return New(Endpoint(DefaultAPIEndpoint))
}

func (c *Client) url(path string) string {
	return fmt.Sprintf("%s/%s", c.endpoint, path)
}
func (c *Client) httpGet(path string) *http.Response {
	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodGet, url, nil))
	req.Header.Add("platform", PlatformHeader)
	req.Header.Add("token", c.token)
	req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36 Edg/105.0.1343.53")
	return unwrap.Err(http.DefaultClient.Do(req))
}
func (c *Client) httpPost(path string, payload []byte) *http.Response {
	params := struct {
		Params string `json:"params"`
	}{Params: string(payload)}

	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodPost, url, bytes.NewReader(unwrap.Err(json.Marshal(params)))))
	req.Header.Add("platform", PlatformHeader)
	req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36 Edg/105.0.1343.53")
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
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}{}

	unwrap.Must(json.Unmarshal(plain, &ok))
	if ok.Code != 200 {
		log.Fatal("get sms code failed, check your phone number please", ok)
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
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	fmt.Println(string(data))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Fatal("login failed: ", ok)
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
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Data    struct {
			SecurityToken   string `json:"securityToken"`
			AccessKeySecret string `json:"accessKeySecret"`
			AccessKeyId     string `json:"accessKeyId"`
		} `json:"data"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))
	if ok.Code != 200 {
		log.Fatal("get oss token failed: ", ok)
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
	// should be less than 30 bytes for the name
	shortize := func(name string) string {
		if len(name) > 30 {
			name = name[:30]
		}
		return name
	}
	params := struct {
		Name   string `json:"name"`
		Singer string `json:"singer"`
		Url    string `json:"url"`
		Type   int    `json:"type"`
		Size   int64  `json:"size"`
	}{
		Name:   shortize(name),
		Singer: singer,
		Url:    url,
		Type:   1,
		Size:   size,
	}

	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)

	resp := c.httpPost("/community/songshare/create", []byte(encoded))
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Fatal("create song share failed: ", ok)
	}

	// add uploaded file to log
	uplog := unwrap.Err(os.OpenFile("smile-upload.log", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644))
	unwrap.Err(uplog.WriteString(name))
	unwrap.Err(uplog.WriteString("\n"))
}

type SongEntry struct {
	Deleted    int    `json:"deleted"`
	CreateTime int    `json:"createTime"`
	UpdateTime int    `json:"updateTime"`
	ID         int    `json:"id"`
	Code       int    `json:"code"`
	Name       string `json:"name"`
	Singer     string `json:"singer"`
	Poster     string `json:"poster"`
	URL        string `json:"url"`
	LyricURL   string `json:"lyricUrl"`
	Type       int    `json:"type"`
	Size       int    `json:"size"`
	UpUID      int    `json:"upuid"`
	PlayCount  int    `json:"playCount"`
	AddCount   int    `json:"addCount"`
	UpNick     string `json:"upnick"`
}

func (c *Client) ListSongs() []SongEntry {
	type OK struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Data    struct {
			Content       []SongEntry `json:"content"`
			Last          bool        `json:"last"`
			TotalPages    int         `json:"totalPages"`
			TotalElements int         `json:"totalElements"`
			NumOfElements int         `json:"numberOfElements"`
		} `json:"data"`
	}
	var songs []SongEntry
	p := 1
	for {
		resp := c.httpGet(fmt.Sprintf("community/songs/users?pageNo=%d&pageSize=100", p))
		data := unwrap.Err(io.ReadAll(resp.Body))
		resp.Body.Close()
		plain := saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data))))

		ok := OK{}
		unwrap.Must(json.Unmarshal(plain, &ok))

		if ok.Code != 200 {
			log.Fatal(ok.Message)
		}

		songs = append(songs, ok.Data.Content...)

		if ok.Data.Last {
			break
		}
		p++
	}
	return songs
}

// {"ids":[3461]}
func (c *Client) RemoveSongs(ids ...int64) {
	params := struct {
		IDs []int64 `json:"ids"`
	}{
		IDs: ids,
	}

	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("/community/songs/remove", []byte(encoded))
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Fatal("create song share failed: ", ok)
	}
}

func (c *Client) CrackRoomPassword(rid string, password string) bool {
	params := struct {
		RoomID   string `json:"rid"`
		Password string `json:"password"`
		RoomInto int    `json:"roomInto"`
		Tab      string `json:"tab"`
		Knock    int    `json:"knock"`
	}{
		RoomID:   rid,
		Password: password,
		RoomInto: 1,
		Tab:      "2-10",
		Knock:    0,
	}

	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)

	resp := c.httpPost("voiceroom/user/joinRoom", []byte(encoded))
	defer resp.Body.Close()

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{}
	data := unwrap.Err(io.ReadAll(resp.Body))
	unwrap.Must(json.Unmarshal(saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data)))), &ok))

	if ok.Code != 200 {
		log.Println(ok)
	}

	return ok.Code == 200
}

type RoomOwner struct {
	ID        int64  `json:"id"`
	Nick      string `json:"nick"`
	Sex       int    `json:"sex"`
	AI        int    `json:"ai"`
	DisplayID int64  `json:"displayId"`
}
type Room struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Locked   int       `json:"locked"`
	Password string    `json:"password"`
	Heat     int       `json:"heat"`
	Owner    RoomOwner `json:"owner"`
}

func (c *Client) ListRooms(tab string, page int) []Room {
	// use default pageSize=10 to act as normal behavior
	q := fmt.Sprintf("tab=%s&pageNo=%d&pageSize=10", tab, page)

	resp := c.httpGet("voiceroom/list/queryRoomByTab?" + q)
	data := unwrap.Err(io.ReadAll(resp.Body))
	resp.Body.Close()
	plain := saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data))))

	ok := struct {
		Code    int
		Message string
		Data    []Room
	}{}

	unwrap.Must(json.Unmarshal(plain, &ok))
	if ok.Code != 200 {
		log.Fatal(ok.Message)
	}

	return ok.Data
}
