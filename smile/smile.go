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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/shafreeck/miao/saes"
	"github.com/shafreeck/miao/unwrap"
)

const DefaultWebEndpoint = "http://www.smilemiao.com/api/v2"
const DefaultAPIEndpoint = "https://api.smilemiao.com/v2"

// endpointKind distinguishes between the web (browser) and app (mobile API) endpoints.
// Each kind may use different URL paths and HTTP headers even when the request
// parameters are identical.
type endpointKind int

const (
	kindWeb endpointKind = iota
	kindApp
)

// Web endpoint paths
const (
	webLoginPath = "community/ucenter"
	webSMSPath   = "community/ucenter/sms"
)

// App endpoint paths — kept separate so they can diverge from the web paths
// independently without touching every call-site.
const (
	appLoginPath = "community/ucenter"
	appSMSPath   = "community/ucenter/sms"
)

type Client struct {
	aesb     cipher.Block
	agent    string
	token    string
	endpoint string
	platform string
	kind     endpointKind
}

type Option func(c *Client)

func Endpoint(ep string) Option {
	return func(c *Client) {
		c.endpoint = ep
	}
}

// AppClient returns an Option that switches the client to app (mobile API) mode,
// setting the correct endpoint, User-Agent, and platform header.
func AppClient() Option {
	return func(c *Client) {
		c.endpoint = DefaultAPIEndpoint
		c.kind = kindApp
	}
}

func New(opts ...Option) *Client {
	c := &Client{
		endpoint: DefaultWebEndpoint,
		kind:     kindWeb,
	}

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

	// set User-Agent and platform header according to the endpoint kind
	switch c.kind {
	case kindApp:
		c.agent = "SmileMiao/2.2.0 (iPhone; iOS 17.3.1; Scale/3.00)"
		c.platform = "ios"
	default: // kindWeb
		c.platform = "pc_web"
		c.agent = "Mozilla/4.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36 Edg/105.0.1343.53"
	}

	return c
}

func NewWebClient() *Client {
	return New()
}

func NewAPIClient() *Client {
	return New(AppClient())
}

// url constructs the full URL for the given path.  Leading slashes on the path
// are stripped to avoid double-slash URLs regardless of how callers format them.
func (c *Client) url(path string) string {
	return fmt.Sprintf("%s/%s", c.endpoint, strings.TrimPrefix(path, "/"))
}

// loginPath returns the correct login endpoint path for this client's kind.
func (c *Client) loginPath() string {
	if c.kind == kindApp {
		return appLoginPath
	}
	return webLoginPath
}

// smsPath returns the correct SMS-code endpoint path for this client's kind.
func (c *Client) smsPath() string {
	if c.kind == kindApp {
		return appSMSPath
	}
	return webSMSPath
}
func (c *Client) httpGet(path string) *http.Response {
	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodGet, url, nil))
	req.Header.Add("platform", c.platform)
	req.Header.Add("token", c.token)
	req.Header.Add("user-agent", c.agent)
	return unwrap.Err(http.DefaultClient.Do(req))
}
func (c *Client) httpPost(path string, payload []byte) *http.Response {
	params := struct {
		Params string `json:"params"`
	}{Params: string(payload)}

	url := c.url(path)
	req := unwrap.Err(http.NewRequest(http.MethodPost, url, bytes.NewReader(unwrap.Err(json.Marshal(params)))))
	req.Header.Add("platform", c.platform)
	req.Header.Add("user-agent", c.agent)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("token", c.token)
	return unwrap.Err(http.DefaultClient.Do(req))

}
func (c *Client) SendSMSCode(phone string) {
	req := struct {
		Phone       string `json:"phone"`
		Command     int    `json:"command"`
		CountryCode string `json:"countryCode"`
		SendTime    int64  `json:"sendTime"`
	}{
		Phone:       phone,
		Command:     6,
		CountryCode: "86",
		SendTime:    time.Now().UnixMilli(),
	}
	rawPayload := unwrap.Err(json.Marshal(req))
	crypted := saes.AESEncrypt(c.aesb, rawPayload)
	encoded := base64.StdEncoding.EncodeToString(crypted)
	
	resp := c.httpPost(c.smsPath(), []byte(encoded))
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
	resp := c.httpPost(c.loginPath(), []byte(encoded))
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
	Count     int    `json:"count"`
	DisplayID int64  `json:"displayId"`
}
type Room struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Locked   int       `json:"locked"`
	Password string    `json:"password"`
	Heat     int       `json:"heat"`
	Owner    RoomOwner `json:"owner"`
	Speakers []struct {
		ID       int64 `json:"id"`
		Role     int   `json:"role"`
		Status   int   `json:"status"`
		Muted    int   `json:"muted"`
		Mic      int   `json:"mic"`
		Miced    int   `json:"miced"`
		Position int   `json:"position"`
	}
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

type User struct {
	ID     int    `json:"id"`
	Nick   string `json:"nick"`
	Avatar string `json:"avatar"`
	Phone  string `json:"phone"`
}

// prettyJSON indents raw JSON for readable CLI output.
func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return string(data)
	}
	return buf.String()
}

// decodeResp reads the HTTP body, AES-decrypts it, and returns the plaintext.
// If the body starts with '{' or '[' it is already plain JSON (e.g. an error
// response) and is returned as-is without base64/AES decoding.
func (c *Client) decodeResp(resp *http.Response) []byte {
	defer resp.Body.Close()
	data := unwrap.Err(io.ReadAll(resp.Body))
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return trimmed
	}
	return saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data))))
}

// BatchQueryUsers queries multiple users by a comma-separated ID string.
// POST /v2/community/chat/batchQueryUsers  {"ids":"123,456"}
func (c *Client) BatchQueryUsers(ids string) []byte {
	params := struct {
		IDs string `json:"ids"`
	}{IDs: ids}
	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("community/chat/batchQueryUsers", []byte(encoded))
	return c.decodeResp(resp)
}

// GetVisitors returns the visitor-change list since the given Unix-millisecond timestamp.
// GET /v2/community/ucenter/visitors/change?time={timestamp}
func (c *Client) GetVisitors(timestamp int64) []byte {
	resp := c.httpGet(fmt.Sprintf("community/ucenter/visitors/change?time=%d", timestamp))
	return c.decodeResp(resp)
}

// extractDataArray unmarshals a JSON response and returns the top-level "data"
// field as a slice of items.
func extractDataArray(plain []byte) ([]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	dataVal, ok := m["data"]
	if !ok {
		return nil, fmt.Errorf("no data field")
	}
	arr, ok := dataVal.([]interface{})
	if !ok {
		// maybe it's under data.content (like in ListSongs)
		if mData, ok := dataVal.(map[string]interface{}); ok {
			if content, ok := mData["content"].([]interface{}); ok {
				return content, nil
			}
			return nil, fmt.Errorf("data is not an array and has no content array")
		}
		return nil, fmt.Errorf("data is not an array, type: %T", dataVal)
	}
	return arr, nil
}

// itemCreateTimeMs returns the createTime (milliseconds) from a feed item,
// checking .daily.createTime first then .createTime.
func itemCreateTimeMs(item map[string]interface{}) int64 {
	if daily, ok := item["daily"].(map[string]interface{}); ok {
		if ct, ok := daily["createTime"].(float64); ok {
			return int64(ct)
		}
	}
	if ct, ok := item["createTime"].(float64); ok {
		return int64(ct)
	}
	return 0
}

// filterItems filters arr to items newer than cutoff and truncates to limit.
func filterItems(arr []interface{}, cutoff int64, collected []interface{}, limit int) ([]interface{}, bool) {
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		t := itemCreateTimeMs(m)
		// If it's a valid timestamp older than our cutoff, we can stop paginating entirely.
		if t > 0 && t < cutoff {
			return collected, true
		}
		// Otherwise, keep the item (including items where we couldn't parse the time, like ads)
		if t >= cutoff || t == 0 {
			collected = append(collected, item)
		}
		if limit > 0 && len(collected) >= limit {
			return collected, true
		}
	}
	return collected, false
}

// GetFeedHot returns the daily hot-list feed filtered by since/limit.
// GET /v2/community/dailies/dailyHotList
func (c *Client) GetFeedHot(since time.Duration, limit int) []byte {
	resp := c.httpGet("community/dailies/dailyHotList")
	plain := c.decodeResp(resp)

	arr, err := extractDataArray(plain)
	if err != nil {
		return plain
	}
	cutoff := time.Now().Add(-since).UnixMilli()
	result, _ := filterItems(arr, cutoff, nil, limit)
	if result == nil {
		result = []interface{}{}
	}
	out, _ := json.Marshal(result)
	return out
}

// GetFeedByUser returns the feed for a specific user filtered by since/limit.
// GET /v2/community/dailies/users/{uid}?pageNo={n}
func (c *Client) GetFeedByUser(uid string, since time.Duration, limit int) []byte {
	cutoff := time.Now().Add(-since).UnixMilli()
	var result []interface{}
	for pageNo := 1; ; pageNo++ {
		resp := c.httpGet(fmt.Sprintf("community/dailies/users/%s?pageNo=%d", uid, pageNo))
		plain := c.decodeResp(resp)
		arr, err := extractDataArray(plain)
		if err != nil || len(arr) == 0 {
			break
		}
		var done bool
		result, done = filterItems(arr, cutoff, result, limit)
		if done {
			break
		}
	}
	if result == nil {
		result = []interface{}{}
	}
	out, _ := json.Marshal(result)
	return out
}

// GetFeedByTab returns the feed for a specific tab filtered by since/limit.
// GET /v2/community/dailies/queryByTab?tabId={tabID}&pageNo={n}
func (c *Client) GetFeedByTab(tabID string, since time.Duration, limit int) []byte {
	cutoff := time.Now().Add(-since).UnixMilli()
	var result []interface{}
	for pageNo := 0; ; pageNo++ {
		resp := c.httpGet(fmt.Sprintf("community/dailies/queryByTab?tabId=%s&pageNo=%d", tabID, pageNo))
		plain := c.decodeResp(resp)
		arr, err := extractDataArray(plain)
		if err != nil || len(arr) == 0 {
			break
		}
		var done bool
		result, done = filterItems(arr, cutoff, result, limit)
		if done {
			break
		}
	}
	if result == nil {
		result = []interface{}{}
	}
	out, _ := json.Marshal(result)
	return out
}

// GetInbox returns the latest chat sessions.
// GET /v2/community/chat/getlatestsession?pageNo=1&pageSize=100&updateTime=0
func (c *Client) GetInbox() []byte {
	resp := c.httpGet("community/chat/getlatestsession?pageNo=1&pageSize=100&updateTime=0")
	return c.decodeResp(resp)
}

// GetCPInfo returns intimate CP info for the given target UID.
// POST /v2/community/intimate/queryCpIntimateFriendInfo {"id":uid}
func (c *Client) GetCPInfo(targetUID string) []byte {
	id, _ := strconv.ParseInt(targetUID, 10, 64)
	params := map[string]interface{}{"id": id}
	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("community/intimate/queryCpIntimateFriendInfo", []byte(encoded))
	return c.decodeResp(resp)
}


// GetFollows returns the follow/follower list for the given user.
// followType=1 for following, followType=2 for followers.
func (c *Client) GetFollows(targetUID string, followType int) []byte {
	var endpoint string
	if followType == 1 {
		endpoint = fmt.Sprintf("community/ucenter/following?pageSize=100&tuid=%s&pageNo=1&keywords=", targetUID)
	} else {
		endpoint = fmt.Sprintf("community/ucenter/followers?pageSize=100&tuid=%s&pageNo=1&keywords=", targetUID)
	}
	resp := c.httpGet(endpoint)
	return c.decodeResp(resp)
}

// SearchUsers searches for users by keyword.
// GET /v2/community/search/user?keyword={keyword}&pageNo={pageNo}&pageSize=20
func (c *Client) SearchUsers(keyword string, pageNo int) []byte {
	resp := c.httpGet(fmt.Sprintf("community/search/user?keyword=%s&pageNo=%d&pageSize=20", url.QueryEscape(keyword), pageNo))
	return c.decodeResp(resp)
}

// SearchTopics searches for topics by keyword.
// POST /v2/community/homepage/search/topic {"keyword":keyword,"pageSize":20,"pageNo":pageNo}
func (c *Client) SearchTopics(keyword string, pageNo int) []byte {
	params := struct {
		Keyword  string `json:"keyword"`
		PageSize int    `json:"pageSize"`
		PageNo   int    `json:"pageNo"`
	}{Keyword: keyword, PageSize: 20, PageNo: pageNo}
	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("community/homepage/search/topic", []byte(encoded))
	return c.decodeResp(resp)
}

// SearchRooms searches for voice rooms by keyword.
// POST /v2/voiceroom/homePage/search/voiceroom {"keyword":keyword,"pageSize":20,"pageNo":pageNo}
func (c *Client) SearchRooms(keyword string, pageNo int) []byte {
	params := struct {
		Keyword  string `json:"keyword"`
		PageSize int    `json:"pageSize"`
		PageNo   int    `json:"pageNo"`
	}{Keyword: keyword, PageSize: 20, PageNo: pageNo}
	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("voiceroom/homePage/search/voiceroom", []byte(encoded))
	return c.decodeResp(resp)
}

// SearchDailies searches for dynamic posts by keyword.
// POST /v2/community/dailies/searchByKeyword {"pageNo":pageNo,"tab":0,"pageSize":10,"keyword":keyword}
func (c *Client) SearchDailies(keyword string, pageNo int) []byte {
	params := struct {
		PageNo   int    `json:"pageNo"`
		Tab      int    `json:"tab"`
		PageSize int    `json:"pageSize"`
		Keyword  string `json:"keyword"`
	}{PageNo: pageNo, Tab: 0, PageSize: 10, Keyword: keyword}
	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)
	resp := c.httpPost("community/dailies/searchByKeyword", []byte(encoded))
	return c.decodeResp(resp)
}

func (c *Client) GetUser(id string) User {
	// build request payload (mirrors the Dart structure)
	params := struct {
		IP        string `json:"ip"`
		Latitude  string `json:"latitude"`
		Longitude string `json:"longitude"`
		Location  string `json:"location"`
		Model     string `json:"model"`
		Platform  int    `json:"platform"`
		Version   string `json:"version"`
	}{
		IP:        "192.168.1.1",
		Latitude:  "0",
		Longitude: "0",
		Location:  "",
		Model:     "",
		Platform:  1,
		Version:   "3.1.3",
	}

	crypted := saes.AESEncrypt(c.aesb, unwrap.Err(json.Marshal(params)))
	encoded := base64.StdEncoding.EncodeToString(crypted)

	resp := c.httpPost("community/ucenter/"+id, []byte(encoded))
	defer resp.Body.Close()

	// decode response
	data := unwrap.Err(io.ReadAll(resp.Body))
	plain := saes.AESDecrypt(c.aesb, unwrap.Err(base64.StdEncoding.DecodeString(string(data))))
	fmt.Println(string(plain))

	ok := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    User   `json:"data"`
	}{}
	unwrap.Must(json.Unmarshal(plain, &ok))

	if ok.Code == 202 { // 登录过期
		log.Fatal("登录过期")
	}
	if ok.Code != 200 {
		log.Fatal("获取用户信息失败: ", ok.Message)
	}

	return ok.Data
}
