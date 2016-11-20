package wechat

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/bitly/go-simplejson"
	"github.com/levigross/grequests"
)

type longPoll struct {
	eqch     chan<- *Event
	rses     *grequests.Session
	rops     *grequests.RequestOptions
	reqState int

	//
	qruuid  string
	qrpic   []byte
	logined bool

	redirUrl    string
	urlBase     string
	pushUrlBase string

	wxdevid      string
	wxuin        string // in cookie
	wxsid        string // in cookie
	wxDataTicket string // in cookie
	wxSKeyOld    string
	wxPassTicket string
	wxuvid       string // in cookie
	wxAuthTicket string // in cookie

	wxSyncKey        *simplejson.Json
	wxSKey           string
	wxInitRawData    string
	wxContactRawData string

	// persistent
	cookies []*http.Cookie
}

func newLongPoll(eqch chan<- *Event) *longPoll {
	this := &longPoll{}
	this.eqch = eqch
	this.wxdevid = "e669767113868187"
	this.rses = grequests.NewSession(nil)
	this.rops = &grequests.RequestOptions{}
	this.rops.Headers = map[string]string{
		"Referer": "https://wx2.qq.com/?lang=en_US",
	}
	this.rops.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.100 Safari/537.36 Vivaldi/1.5.658.31"

	this.rops.RequestTimeout = 5 * time.Second

	this.loadCookies()
	if this.cookies != nil {
		this.rops.Cookies = this.cookies
	}

	return this
}

func (this *longPoll) start() {
	go this.run()
}

const (
	REQ_NONE int = iota
	REQ_LOGIN
	REQ_QRCODE
	REQ_WAIT_SCAN
	REQ_REDIR_LOGIN
	REQ_WXINIT
	REQ_CONTACT
	REQ_SYNC_CHECK
	REQ_WEB_SYNC
	REQ_END
)

// blocked
func (this *longPoll) run() {
	log.Println("polling...")
	stopped := false
	if this.cookies != nil {
		this.reqState = REQ_SYNC_CHECK
	} else {
		this.reqState = REQ_LOGIN
	}

	for !stopped {
		switch this.reqState {
		case REQ_LOGIN:
			this.jslogin()
		case REQ_QRCODE:
			this.getqrcode()
		case REQ_WAIT_SCAN:
			this.pollScan()
		case REQ_REDIR_LOGIN:
			this.redirLogin()
		case REQ_WXINIT:
			this.wxInit()
		case REQ_CONTACT:
			this.getContact()
		case REQ_SYNC_CHECK:
			this.syncCheck()
		case REQ_WEB_SYNC:
			this.webSync()
		case REQ_END:
			stopped = true
		}
	}

	log.Println("run end")
}

func parseuuid(str string) (code int, uuid string) {
	exp := regexp.MustCompile(`window.QRLogin.code = (\d+); window.QRLogin.uuid = "([\w\-=]+)";`)
	mats := exp.FindAllStringSubmatch(str, -1)
	log.Println(mats, exp.MatchString(str))
	if len(mats) > 0 {
		code, _ = strconv.Atoi(mats[0][1])
		uuid = mats[0][2]
	}
	return
}

func (this *longPoll) saveContent(name string, bcc []byte, resp *grequests.Response, url string) {
	err := ioutil.WriteFile(name, bcc, 0644)
	if err != nil {
		log.Println(err)
	}
}

func (this *longPoll) loadCookies() {
	sck, err := ioutil.ReadFile("cookies.txt")
	if err != nil {
		log.Println(err)
	} else {
		jck, err := simplejson.NewJson(sck)
		if err != nil {
			log.Println(err)
		} else {
			ckarr := jck.Get("cookies").MustArray()
			if len(ckarr) == 0 {
				log.Println("Invalid json node")
				return
			}
			this.qruuid = jck.Get("qruuid").MustString()
			this.wxSKey = jck.Get("wxskey").MustString()
			this.wxPassTicket = jck.Get("pass_ticket").MustString()
			this.redirUrl = jck.Get("redir_url").MustString()
			this.urlBase = jck.Get("urlBase").MustString()
			this.pushUrlBase = jck.Get("pushUrlBase").MustString()
			this.wxSyncKey = jck.Get("SyncKey")

			this.cookies = make([]*http.Cookie, 0)
			for idx, _ := range ckarr {
				hck := &http.Cookie{}
				bck, err := jck.Get("cookies").GetIndex(idx).MarshalJSON()
				if err != nil {
				}
				err = json.Unmarshal(bck, hck)
				if err != nil {
					log.Println(err)
				} else {
					log.Println(idx, hck)
					this.cookies = append(this.cookies, hck)
					if hck.Name == "wxuin" {
						this.wxuin = hck.Value
					} else if hck.Name == "wxsid" {
						this.wxsid = hck.Value
					} else if hck.Name == "webwx_data_ticket" {
						this.wxDataTicket = hck.Value
					} else if hck.Name == "webwxuvid" {
						this.wxuvid = hck.Value
					} else if hck.Name == "webwx_auth_ticket" {
						this.wxAuthTicket = hck.Value
					}
				}
			}
		}
	}
}

func (this *longPoll) saveCookies(resp *grequests.Response) {
	/*
		for _, ck := range resp.RawResponse.Cookies() {
			log.Println(ck)
		}
		for hkey, hval := range resp.Header {
			log.Println(hkey, "=", hval)
		}
	*/

	var jck *simplejson.Json
	bck, err := ioutil.ReadFile("cookies.txt")
	if err != nil {
		log.Println(err)
		jck = simplejson.New()
	} else {
		jck, err = simplejson.NewJson(bck)
	}

	if len(resp.RawResponse.Cookies()) > 0 {
		// 合并cookies：解释，添加
		ckarr := jck.Get("cookies").MustArray()
		cookies := make([]*http.Cookie, 0)
		for idx, _ := range ckarr {
			hck := &http.Cookie{}
			bck, err := jck.Get("cookies").GetIndex(idx).MarshalJSON()
			if err != nil {
			}
			err = json.Unmarshal(bck, hck)
			cookies = append(cookies, hck)
		}
		newcookies := resp.RawResponse.Cookies()
		for _, ckold := range cookies {
			found := false
			for _, ck := range newcookies {
				if ckold.Name == ck.Name {
					found = true
					break
				}
			}
			if !found { // refill it
				newcookies = append(newcookies, ckold)
			}
		}
		jck.Set("cookies", newcookies)
	}
	jck.Set("qruuid", this.qruuid)
	jck.Set("wxskey", this.wxSKey)
	jck.Set("pass_ticket", this.wxPassTicket)
	jck.Set("redir_url", this.redirUrl)
	jck.Set("urlBase", this.urlBase)
	jck.Set("pushUrlBase", this.pushUrlBase)
	jck.Set("SyncKey", this.wxSyncKey)

	sck, err := jck.Encode()
	if err != nil {
		log.Println(err)
	} else {
		log.Println(string(sck))
		sck, err = jck.EncodePretty()
		this.saveContent("cookies.txt", sck, nil, "")
	}
}

var ips = strings.Split("101.226.76.164 101.227.160.102 140.206.160.161 140.207.135.104 117.135.169.34 117.144.242.33 203.205.151.221", " ")

func (this *longPoll) jslogin() {
	url := "https://login.weixin.qq.com/jslogin?appid=wx782c26e4c19acffb&redirect_uri=https%3A%2F%2Fwx2.qq.com%2Fcgi-bin%2Fmmwebwx-bin%2Fwebwxnewloginpage&fun=new&lang=en_US"
	log.Println(url)
	resp, err := this.rses.Get(url, this.rops)
	if err != nil {
		log.Println(err, url)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("jslogin.html", bcc, resp, url)
	defer resp.Close()

	// # parse hcc: window.QRLogin.code = 200; window.QRLogin.uuid = "gYmgd1grLg==";
	code, uuid := parseuuid(resp.String())
	if code != 200 {
		log.Println(resp.String())
		this.reqState = REQ_END
	} else {
		this.qruuid = uuid
		this.reqState = REQ_QRCODE
		// this.saveCookies(resp)
		this.eqch <- newEvent(EVT_GOT_UUID, []string{uuid})
	}

}

func (this *longPoll) getqrcode() {
	nsurl := "https://login.weixin.qq.com/qrcode/4ZYgra8RHw=="
	nsurl = "https://login.weixin.qq.com/qrcode/" + this.qruuid
	log.Println(nsurl)

	resp, err := this.rses.Get(nsurl, this.rops)
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("qrcode.jpg", bcc, resp, nsurl)
	defer resp.Close()
	this.qrpic = bcc

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		this.reqState = REQ_WAIT_SCAN
		// this.saveCookies(resp)
		this.eqch <- newEvent(EVT_GOT_QRCODE, []string{resp.String()})

	}

}

func (this *longPoll) nowTime() int64 {
	return time.Now().Unix()
}

func parsescan(str string) (code int) {
	exp := regexp.MustCompile(`window.code=(\d+);`)
	mats := exp.FindAllStringSubmatch(str, -1)
	if len(mats) > 0 {
		code, _ = strconv.Atoi(mats[0][1])
	}
	return
}

func (this *longPoll) pollScan() {
	nsurl := "https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=4eDUw9zdPg==&tip=0&r=-1166218796"
	// # v2 url: https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=gfNC8TeiPg==&tip=1&r=-1222670084&lang=en_US
	nsurl = fmt.Sprintf("https://login.weixin.qq.com/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=%s&tip=0&r=%d&lang=en_US&_=%d",
		this.qruuid, this.nowTime(), this.nowTime())
	log.Println(nsurl)

	resp, err := this.rses.Get(nsurl, this.rops)
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Ok, resp.Header, resp.String())
	bcc := resp.Bytes()
	this.saveContent("pollscan.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		/*
					# window.code=408;  # 像是超时
					# window.code=400;  # ??? 难道是会话过期???需要重新获取QR图（已确认，在浏览器中，收到400后刷新了https://wx2.qq.com/
			            # window.code=201;  # 已扫描，未确认
			            # window.code=200;  # 已扫描，已确认登陆
			            # parse hcc, format: window.code=201;
		*/
		code := parsescan(resp.String())

		switch code {
		case 408:
			this.reqState = REQ_WAIT_SCAN // no change
		case 400:
			log.Println("maybe need rerun refresh()...")
			this.reqState = REQ_END
		case 201:
			time.Sleep(2 * time.Second)
		case 200:
			this.logined = true
			this.redirUrl = strings.Split(resp.String(), "\"")[1]
			this.reqState = REQ_REDIR_LOGIN
		default:
			log.Println("not impled", code)
			this.reqState = REQ_END
		}

		// this.saveCookies(resp)
		this.eqch <- newEvent(EVT_GOT_QRCODE, []string{fmt.Sprintf("%d", code)})

	}

}

func parseTicket(str string) (ret int, skey string, wxsid string,
	wxuin string, pass_ticket string) {
	// `<error><ret>0</ret><message>OK</message><skey>@crypt_3ea2fe08_723d1e1bd7b4171657b58c6d2849b367</skey><wxsid>9qxNHGgi9VP4/Tx6</wxsid><wxuin>979270107</wxuin><pass_ticket>%2BEdqKi12tfvM8ZZTdNeh4GLO9LFfwKLQRpqWk8LRYVWFkDE6%2FZJJXurz79ARX%2FIT</pass_ticket><isgrayscale>1</isgrayscale></error>`
	exp := regexp.MustCompile(`<error><ret>(\d+)</ret><message>.*</message><skey>(.+)</skey><wxsid>(.+)</wxsid><wxuin>(\d+)</wxuin><pass_ticket>(.+)</pass_ticket><isgrayscale>1</isgrayscale></error>`)
	mats := exp.FindAllStringSubmatch(str, -1)
	if len(mats) > 0 {
		ret, _ = strconv.Atoi(mats[0][1])
		skey = mats[0][2]
		wxsid = mats[0][3]
		wxuin = mats[0][4]
		pass_ticket = mats[0][5]
	}
	log.Println(mats)
	return
}

func (this *longPoll) redirLogin() {
	nsurl := this.redirUrl + "&fun=new&version=v2"
	if strings.Contains(nsurl, "wx.qq.com") {
		this.urlBase = "https://wx.qq.com"
		this.pushUrlBase = "https://webpush.weixin.qq.com"
	} else {
		this.urlBase = "https://wx2.qq.com"
		this.pushUrlBase = "https://webpush2.weixin.qq.com"
	}
	log.Println(nsurl)

	resp, err := this.rses.Get(nsurl, this.rops)
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("redir.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		/*
			# parse content: SKey,pass_ticket
			# <error><ret>0</ret><message>OK</message><skey>@crypt_3ea2fe08_723d1e1bd7b4171657b58c6d2849b367</skey><wxsid>9qxNHGgi9VP4/Tx6</wxsid><wxuin>979270107</wxuin><pass_ticket>%2BEdqKi12tfvM8ZZTdNeh4GLO9LFfwKLQRpqWk8LRYVWFkDE6%2FZJJXurz79ARX%2FIT</pass_ticket><isgrayscale>1</isgrayscale></error>
		*/
		var ret int = -1
		ret, this.wxSKey, this.wxsid, this.wxuin, this.wxPassTicket =
			parseTicket(resp.String())
		if ret != 0 {
			log.Println("failed")
		}
		this.reqState = REQ_WXINIT
		this.saveCookies(resp)
		this.eqch <- newEvent(EVT_REDIR_URL, []string{nsurl})
	}

}

func (this *longPoll) wxInit() {
	// # TODO: pass_ticket参数
	// nsurl = 'https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit?r=1377482058764'
	// # v2 url:https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit?r=-1222669677&lang=en_US&pass_ticket=%252BEdqKi12tfvM8ZZTdNeh4GLO9LFfwKLQRpqWk8LRYVWFkDE6%252FZJJXurz79ARX%252FIT
	// #nsurl = 'https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit?r=%s&lang=en_US&pass_ticket=' % \
	// #        (self.nowTime() - 3600 * 24 * 30)
	// #nsurl = self.urlBase + '/cgi-bin/mmwebwx-bin/webwxinit?r=%s&lang=en_US&pass_ticket=' % \
	// nsurl = self.urlBase + '/cgi-bin/mmwebwx-bin/webwxinit?r=%s&lang=en_US&pass_ticket=%s' % \
	// (self.nowTime() - 3600 * 24 * 30, self.wxPassTicket)
	// qDebug(nsurl)
	nsurl := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxinit?r=%d&lang=en_US&pass_ticket=%s",
		this.urlBase, time.Now().Unix()-3600*24*30, this.wxPassTicket)

	/*
		post_data = '{"BaseRequest":{"Uin":"%s","Sid":"%s","Skey":"","DeviceID":"%s"}}' % \
		(self.wxuin, self.wxsid, self.devid)

		req = requests.Request('post', nsurl, data=post_data.encode())
	*/

	postData := fmt.Sprintf(`{"BaseRequest":{"Uin":"%s","Sid":"%s","Skey":"","DeviceID":"%s"}}`,
		this.wxuin, this.wxsid, this.wxdevid)
	this.rops.JSON = postData

	resp, err := this.rses.Post(nsurl, this.rops)
	this.rops.JSON = nil
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("wxinit.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		jcc, err := simplejson.NewJson(bcc)
		if err != nil {
			log.Println(err)
		} else {
			ret := jcc.GetPath("BaseResponse", "Ret").MustInt()
			log.Println("ret", ret)
			switch ret {
			case 1101:
				this.reqState = REQ_END
			default:
				this.wxSyncKey = jcc.Get("SyncKey")
				this.wxSKeyOld = this.wxSKey
				this.wxSKey = jcc.Get("SKey").MustString()
				if this.wxSKey != this.wxSKeyOld {
					log.Println("SKey updated:", this.wxSKeyOld, this.wxSKey)
				}
				this.wxInitRawData = resp.String()
				this.reqState = REQ_SYNC_CHECK
				this.saveCookies(resp)
			}
		}
	}

}

func (this *longPoll) getContact() {

	// nsurl = 'https://wx.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact?r=1377482079876'
	// #nsurl = 'https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact?r='
	// nsurl = self.urlBase + '/cgi-bin/mmwebwx-bin/webwxgetcontact?r='
	nsurl := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxgetcontact?r=", this.urlBase)

	/*
		post_data = '{}'
		req = requests.Request('post', nsurl, data=post_data.encode())
	*/

	postData := fmt.Sprintf(`{}`)
	this.rops.JSON = postData

	resp, err := this.rses.Post(nsurl, this.rops)
	this.rops.JSON = nil
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("wxcontact.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {

		jcc, err := simplejson.NewJson(bcc)
		if err != nil {
			log.Println(err)
		} else {
			ret := jcc.GetPath("BaseResponse", "Ret").MustInt()
			log.Println("ret", ret)
			switch ret {
			case 1101:
				this.reqState = REQ_END
			default:
				this.wxContactRawData = resp.String()
				this.reqState = REQ_SYNC_CHECK
				// this.saveCookies(resp)
			}
		}
	}
}

func (this *longPoll) packSyncKey() string {
	/*
		### make syncKey: format: 1_124125|2_452346345|3_65476547|1000_5643635
		syncKey = []
		for k in self.wxSyncKey['List']:
		elem = '%s_%s' % (k['Key'], k['Val'])
		syncKey.append(elem)

		# |需要URL编码成%7C
		syncKey = '%7C'.join(syncKey)   # [] => str''
	*/

	count := this.wxSyncKey.Get("Count").MustInt()
	log.Println("count:", count)
	skarr := make([]string, 0)
	for idx := 0; idx < count; idx++ {
		key := this.wxSyncKey.Get("List").GetIndex(idx).Get("Key").MustInt()
		val := this.wxSyncKey.Get("List").GetIndex(idx).Get("Val").MustInt()
		skarr = append(skarr, fmt.Sprintf("%d_%d", key, val))
	}
	return strings.Join(skarr, "%7C")
}

func parsesynccheck(str string) (code int, selector int) {
	// window.synccheck={retcode:"1101",selector:"0"}
	exp := regexp.MustCompile(`window.synccheck={retcode:"(\d+)",selector:"(\d+)"}`)
	mats := exp.FindAllStringSubmatch(str, -1)
	if len(mats) > 0 {
		code, _ = strconv.Atoi(mats[0][1])
		selector, _ = strconv.Atoi(mats[0][2])
	} else {
		code = -1
		selector = -1
	}
	return
}

func (this *longPoll) dumpState() {

	log.Println("qruuid		 ", this.qruuid)
	log.Println("qrpic		 ", len(this.qrpic))
	log.Println("logined	 ", this.logined)
	log.Println("redirUrl	 ", this.redirUrl)
	log.Println("urlBase	 ", this.urlBase)
	log.Println("pushUrlBase ", this.pushUrlBase)
	log.Println("wxdevid	 ", this.wxdevid)
	log.Println("wxuin		 ", this.wxuin)
	log.Println("wxsid		 ", this.wxsid)
	log.Println("wxDataTicket", this.wxDataTicket)
	log.Println("wxSKeyOld	 ", this.wxSKeyOld)
	log.Println("wxPassTicket", this.wxPassTicket)
	log.Println("wxuvid		 ", this.wxuvid)
	log.Println("wxAuthTicket", this.wxAuthTicket)
	log.Println("wxSyncKey	 ", this.wxSyncKey)
	log.Println("wxSKey		 ", this.wxSKey)
	log.Println("wxInitRawData  ", len(this.wxInitRawData))
	log.Println("wxContactRawData", len(this.wxContactRawData))

}

func (this *longPoll) syncCheck() {
	syncKey := this.packSyncKey()
	skey := strings.Replace(this.wxSKey, "@", "%40", -1)
	log.Println(this.wxSKey, "=>", skey)
	pass_ticket := strings.Replace(this.wxPassTicket, "%", "%25", -1)
	nsurl := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/synccheck?r=%d&skey=%s&sid=%s&uin=%s&deviceid=%s&synckey=%s&lang=en_US&pass_ticket=%s",
		this.pushUrlBase, this.nowTime(), skey, this.wxsid, this.wxuin,
		this.wxdevid, syncKey, pass_ticket)
	log.Println("requesting...", nsurl)

	resp, err := this.rses.Get(nsurl, this.rops)
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("synccheck.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		log.Println(resp.String())
		retcode, selector := parsesynccheck(resp.String())

		switch retcode {
		case -1:
			log.Fatalln("wtf")
		case 1100:
			log.Println("maybe need reget SyncKey, rerun wxinit() ...")
		case 1101:
			this.dumpState()
			log.Println("maybe need rerun relogin...", resp.String())
			this.reqState = REQ_END
		case 0:
			switch selector {
			case 0: // go on syncCheck
			case 1:
				fallthrough
			case 2:
				fallthrough
			case 4:
				fallthrough
			case 5:
				fallthrough
			case 6:
				fallthrough
			case 7:
				this.reqState = REQ_WEB_SYNC
			default:
				log.Println("unknown selector:", retcode, selector)
			}
		default:
			log.Println("error sync check ret code:", retcode, selector)
		}
	}
}

func (this *longPoll) webSync() {
	skey := strings.Replace(this.wxSKey, "@", "%40", -1)
	log.Println(this.wxSKey, "=>", skey)
	pass_ticket := strings.Replace(this.wxPassTicket, "%", "%25", -1)
	nsurl := fmt.Sprintf("%s/cgi-bin/mmwebwx-bin/webwxsync?sid=%s&skey=%s&lang=en_US&pass_ticket=%s", this.urlBase, this.wxsid, skey, pass_ticket)
	BaseRequest := map[string]string{
		"Uin":      this.wxuin,
		"Sid":      this.wxsid,
		"SKey":     this.wxSKey,
		"DeviceID": this.wxdevid}
	post_data_obj := simplejson.New()
	post_data_obj.Set("BaseRequest", BaseRequest)
	post_data_obj.Set("SyncKey", this.wxSyncKey)
	post_data_obj.Set("rr", this.nowTime())

	post_data_bin, err := post_data_obj.Encode()
	if err != nil {
		log.Println(err)
	}
	post_data := string(post_data_bin)

	this.rops.JSON = post_data
	this.rops.Headers["Content-Type"] = "application/x-www-form-urlencoded"
	resp, err := this.rses.Post(nsurl, this.rops)
	this.rops.JSON = nil
	delete(this.rops.Headers, "Content-Type")
	if err != nil {
		log.Println(err, nsurl)
	}
	log.Println(resp.StatusCode, resp.Header, resp.Ok)
	bcc := resp.Bytes()
	this.saveContent("websync.html", bcc, resp, nsurl)
	defer resp.Close()

	if !resp.Ok {
		this.reqState = REQ_END
	} else {
		jcc, err := simplejson.NewJson(bcc)
		if err != nil {
			log.Println(jcc)
			this.reqState = REQ_END
		} else {
			if jcc.GetPath("SyncKey", "Count").MustInt() == 0 {
				log.Println("websync's SyncKey empty, maybe need refresh...")
				this.reqState = REQ_END
			} else {
				// update SyncKey and SKey
				this.wxSyncKey = jcc.Get("SyncKey")

				// check data
				ret := jcc.GetPath("BaseResponse", "Ret").MustInt()
				switch ret {
				case 0:
					this.reqState = REQ_SYNC_CHECK
					this.saveCookies(resp)
				case 1101:
					log.Println("maybe need rerun refresh()...1101")
					this.reqState = REQ_END
				case -1:
					log.Println("wtf")
					this.reqState = REQ_END
				default:
					log.Println("web sync error:", ret)
					this.reqState = REQ_END
				}
			}
		}
	}

}
