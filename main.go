package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

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

func smileEncodeCommand() {
	args := struct {
		Text string
	}{}
	cortana.Parse(&args)

	plain := []byte(args.Text)
	if len(plain) == 0 {
		plain = bytes.TrimSpace(unwrap.Err(io.ReadAll(os.Stdin)))
	}

	b, err := aes.NewCipher(saes.AESKey)
	if err != nil {
		log.Fatal(err)
	}

	data := saes.AESEncrypt(b, plain)
	fmt.Println(base64.StdEncoding.EncodeToString(data))
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

func smileSearchCommand() {
	args := struct {
		Keyword string `cortana:"keyword, -, -"`
		User    bool   `cortana:"--user"`
		Room    bool   `cortana:"--room"`
		Topic   bool   `cortana:"--topic"`
		Feed    bool   `cortana:"--feed"`
		Page    int    `cortana:"--page, -p, 1"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	results := make(map[string]interface{})

	addResult := func(key string, plain []byte) {
		var val interface{}
		if err := json.Unmarshal(plain, &val); err != nil {
			results[key] = string(plain)
		} else {
			results[key] = val
		}
	}

	isIntegrated := !args.User && !args.Room && !args.Topic && !args.Feed

	if isIntegrated {
		type res struct {
			key string
			val []byte
		}
		ch := make(chan res, 4)
		go func() { ch <- res{"users", sm.SearchUsers(args.Keyword, args.Page)} }()
		go func() { ch <- res{"rooms", sm.SearchRooms(args.Keyword, args.Page)} }()
		go func() { ch <- res{"topics", sm.SearchTopics(args.Keyword, args.Page)} }()
		go func() { ch <- res{"feeds", sm.SearchDailies(args.Keyword, args.Page)} }()

		for i := 0; i < 4; i++ {
			r := <-ch
			addResult(r.key, r.val)
		}
	} else {
		if args.User {
			addResult("users", sm.SearchUsers(args.Keyword, args.Page))
		}
		if args.Room {
			addResult("rooms", sm.SearchRooms(args.Keyword, args.Page))
		}
		if args.Topic {
			addResult("topics", sm.SearchTopics(args.Keyword, args.Page))
		}
		if args.Feed {
			addResult("feeds", sm.SearchDailies(args.Keyword, args.Page))
		}
	}

	b, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(b))
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
	var files []os.DirEntry
	if _, err := os.Stat("bgm/"); err == nil {
		files = unwrap.Err(os.ReadDir("bgm/"))
	}
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

func crackRoomPassword() {
	args := struct {
		Delay    time.Duration `cortana:"--delay, -, 100ms"`
		RoomID   string        `cortana:"--rid, -, -"`
		Password string        `cortana:"password"`
		DictFile string        `cortana:"--dict"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()

	if args.Password == "" && args.DictFile != "" {
		dict := unwrap.Err(os.Open(args.DictFile))
		scanner := bufio.NewScanner(dict)
		for scanner.Scan() {
			password := scanner.Text()
			if sm.CrackRoomPassword(args.RoomID, password) {
				fmt.Println("密码是：", password)
				return
			}
			time.Sleep(args.Delay)
		}
	} else if args.Password != "" {
		if sm.CrackRoomPassword(args.RoomID, args.Password) {
			fmt.Println("密码是：", args.Password)
			return
		}
	} else {
		cortana.Usage()
		return
	}
	fmt.Println("密码错误")
}

func listRooms() {
	args := struct {
		Tab   string        `cortana:"--tab, -,"`
		Page  int           `cortana:"--page, -p, 1"`
		All   bool          `cortana:"--all"`
		Delay time.Duration `cortana:"--delay, -, 1s"`
		Value string        `cortana:"<value>,, 日常"`
	}{}
	cortana.Parse(&args)

	value2Tab := map[string]string{
		"我的关注": "3-0",
		"关注":   "3-0",
		"附近":   "2-1000",
		"发现":   "2-0",
		"日常":   "2-10",
		"综艺":   "2-11",
		"唱见":   "2-12",
		"萌新":   "2-17",
		"游戏":   "2-15",
		"速配":   "1-15",
		"情感":   "1-16",
		"拍拍":   "1-17",
	}
	if args.Value != "" && args.Tab == "" {
		args.Tab = value2Tab[args.Value]
	}
	sm := smile.NewWebClient()
	for p := args.Page; ; p++ {
		rooms := sm.ListRooms(args.Tab, p)
		for _, room := range rooms {
			data := unwrap.Err(json.Marshal(room))
			fmt.Println(string(data))
		}
		if !args.All {
			break
		}
		if len(rooms) == 0 {
			break
		}
		time.Sleep(args.Delay)
	}

}

func getUserCommand() {
	args := struct {
		ID string `cortana:"id, -, auto"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	u := sm.GetUser(args.ID)
	b := unwrap.Err(json.Marshal(u))
	fmt.Println(string(b))
}

func getUsersCommand() {
	args := struct {
		IDs string `cortana:"ids, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	plain := sm.BatchQueryUsers(args.IDs)
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func getVisitorsCommand() {
	args := struct {
		Time int64 `cortana:"--time, -, 0"`
	}{}
	cortana.Parse(&args)

	ts := args.Time
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	sm := smile.NewWebClient()
	plain := sm.GetVisitors(ts)
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days, _ := strconv.Atoi(strings.TrimSuffix(s, "d"))
		return time.Duration(days) * 24 * time.Hour
	}
	if strings.HasSuffix(s, "w") {
		weeks, _ := strconv.Atoi(strings.TrimSuffix(s, "w"))
		return time.Duration(weeks) * 7 * 24 * time.Hour
	}
	if strings.HasSuffix(s, "y") {
		years, _ := strconv.Atoi(strings.TrimSuffix(s, "y"))
		return time.Duration(years) * 365 * 24 * time.Hour
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 48 * time.Hour
	}
	return d
}

func getFeedCommand() {
	args := struct {
		Hot   bool   `cortana:"--hot"`
		UID   string `cortana:"--uid"`
		Tab   string `cortana:"--tab, -, , 发现|最新|找搭子|日常|游戏|萌新"`
		Since string `cortana:"--since, -, 0, 时间范围(支持h/d/w/y, 如3d, 1y; 0表示不过滤)"`
		Limit int    `cortana:"--limit, -, 50"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	var plain []byte
	sinceDur := parseDuration(args.Since)
	switch {
	case args.Hot:
		plain = sm.GetFeedHot(sinceDur, args.Limit)
	case args.UID != "":
		plain = sm.GetFeedByUser(args.UID, sinceDur, args.Limit)
	case args.Tab != "":
		tabID := args.Tab
		switch args.Tab {
		case "发现":
			tabID = "1"
		case "最新":
			tabID = "2"
		case "找搭子":
			tabID = "3"
		case "日常":
			tabID = "4"
		case "游戏":
			tabID = "5"
		case "萌新":
			tabID = "6"
		}
		plain = sm.GetFeedByTab(tabID, sinceDur, args.Limit)
	default:
		cortana.Usage()
		return
	}

	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func inboxCommand() {
	sm := smile.NewWebClient()
	plain := sm.GetInbox()
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func cpCommand() {
	args := struct {
		UID string `cortana:"uid, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	plain := sm.GetCPInfo(args.UID)
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func followsCommand() {
	args := struct {
		UID  string `cortana:"uid, -, -"`
		Fans bool   `cortana:"--fans"`
	}{}
	cortana.Parse(&args)

	followType := 1
	if args.Fans {
		followType = 2
	}

	sm := smile.NewWebClient()
	plain := sm.GetFollows(args.UID, followType)
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func postCommand() {
	args := struct {
		Content []string `cortana:"content, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	plain := sm.PostDaily(strings.Join(args.Content, " "))
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func rmDailyCommand() {
	args := struct {
		ID string `cortana:"id, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	plain := sm.RemoveDaily(args.ID)
	var buf bytes.Buffer
	json.Indent(&buf, plain, "", "  ")
	fmt.Println(buf.String())
}

func msgSendCommand() {
	args := struct {
		ToUID string   `cortana:"toUid, -, -"`
		Text  []string `cortana:"text, -, -"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	plain := sm.SendMessage(args.ToUID, strings.Join(args.Text, " "), 0)
	// try to parse as JSON, if fails just print raw
	var buf bytes.Buffer
	if err := json.Indent(&buf, plain, "", "  "); err != nil {
		fmt.Println(string(plain))
		return
	}
	fmt.Println(buf.String())
}

func msgRecvCommand() {
	args := struct {
		Since string `cortana:"--since, -s, 1h, 时间范围(支持h/d/w/y, 如3d, 1h; 0表示不过滤)"`
	}{}
	cortana.Parse(&args)

	sm := smile.NewWebClient()
	var startTime int64
	if args.Since != "0" && args.Since != "" {
		dur := parseDuration(args.Since)
		startTime = time.Now().Add(-dur).UnixMilli()
	}
	plain := sm.GetMessages(startTime)

	// Decrypt the msgData field (double-encrypted) in each message before printing.
	var resp map[string]interface{}
	if err := json.Unmarshal(plain, &resp); err == nil {
		if dataVal, ok := resp["data"].(map[string]interface{}); ok {
			if msgs, ok := dataVal["msg_list"].([]interface{}); ok {
				b, err := aes.NewCipher(saes.AESKey)
				if err == nil {
					for _, item := range msgs {
						if m, ok := item.(map[string]interface{}); ok {
							if msgData, ok := m["msg_data"].(string); ok {
								if decoded, err := base64.StdEncoding.DecodeString(msgData); err == nil {
									m["msg_data"] = string(saes.AESDecrypt(b, decoded))
								}
							}
						}
					}
				}
			}
		}
	}
	out, err := json.MarshalIndent(resp, "", "  ")
	if err == nil {
		fmt.Println(string(out))
		return
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, plain, "", "  "); err != nil {
		fmt.Println(string(plain))
		return
	}
	fmt.Println(buf.String())
}

func main() {
	cortana.AddRootCommand(downloadAndUploadCommand)
	cortana.AddCommand("download", xijingDownloadCommand, "从戏鲸下载BGM")
	cortana.AddCommand("search-bgm", xijingSearchCommand, "从戏鲸搜索BGM")
	cortana.AddCommand("search", smileSearchCommand, "四喵综合搜索 (--user | --room | --topic | --feed)")
	cortana.AddCommand("decode", smileDecodeCommand, "解码四喵密文")
	cortana.AddCommand("encode", smileEncodeCommand, "编码四喵明文")
	cortana.AddCommand("sms", smileSendSMSCommand, "发送短信验证码")
	cortana.AddCommand("login", smileLoginCommand, "登录四喵")
	cortana.AddCommand("oss upload", smileUploadOSSCommand, "上传 MP3 文件到 OSS")
	cortana.AddCommand("upload", smileUploadCommand, "上传 MP3 文件到 OSS 并创建分享")
	cortana.AddCommand("mp3codec", mp3codeCommand, "把音频文件转为 mp3 格式")
	cortana.AddCommand("miao", miaoCommand, "列出已上传的文件")
	cortana.AddCommand("segment", segmentCommand, "分割 MP3")
	cortana.AddCommand("switch", switchUserCommand, "切换用户")
	cortana.AddCommand("remove", removeSongsCommand, "删除歌曲")
	cortana.AddCommand("crack room", crackRoomPassword, "破解密码")
	cortana.AddCommand("list rooms", listRooms, "列出直播间")
	cortana.AddCommand("get user", getUserCommand, "获取用户信息")
	cortana.AddCommand("users", getUsersCommand, "批量查询用户信息")
	cortana.AddCommand("visitors", getVisitorsCommand, "获取访客列表")
	cortana.AddCommand("feed", getFeedCommand, "获取动态/Feed (--hot | --uid <id> | --tab <发现|最新|找搭子|日常|游戏|萌新>)")
	cortana.AddCommand("inbox", inboxCommand, "获取最新聊天会话列表")
	cortana.AddCommand("cp", cpCommand, "获取亲密CP信息")
	cortana.AddCommand("follows", followsCommand, "获取关注列表 (--fans 切换为粉丝列表)")
	cortana.AddCommand("post", postCommand, "发布日常动态")
	cortana.AddCommand("rmdaily", rmDailyCommand, "删除日常动态")
	cortana.AddCommand("msg send", msgSendCommand, "发送私信消息")
	cortana.AddCommand("msg recv", msgRecvCommand, "接收最新私信消息")
	cortana.Launch()
}
