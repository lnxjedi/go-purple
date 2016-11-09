/*
   tox-prpl core code, minimal must implemention code.
*/
package main

import (
	"encoding/base64"
	"log"
	"math/rand"
	"strconv"
	"time"

	"yobot/purple"

	"github.com/kitech/colog"
	"github.com/kitech/go-toxcore"
)

type ToxPlugin struct {
	ppi      *purple.PluginProtocolInfo
	pi       *purple.PluginInfo
	p        *purple.Plugin
	_tox     *tox.Tox
	_toxav   *tox.ToxAV
	_toxopts *tox.ToxOptions
	stopch   chan struct{}
}

// plugin functions
func (this *ToxPlugin) init_tox(p *purple.Plugin) {
	log.Println("called")

}

func (this *ToxPlugin) load_tox(p *purple.Plugin) bool {
	log.Println("called")
	rand.Seed(time.Now().UnixNano())
	return true
}

func (this *ToxPlugin) unload_tox(p *purple.Plugin) bool {
	log.Println("called")
	return true
}

func (this *ToxPlugin) destroy_tox(p *purple.Plugin) {
	log.Println("called")
}

// protocol functions
func (this *ToxPlugin) tox_blist_icon() string {
	log.Println("called")
	return "gotox"
}

var bsnodes = []string{
	"biribiri.org", "33445", "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67",
	"178.62.250.138", "33445", "788236D34978D1D5BD822F0A5BEBD2C53C64CC31CD3149350EE27D4D9A2F9B6B",
	"205.185.116.116", "33445", "A179B09749AC826FF01F37A9613F6B57118AE014D4196A0E1105A98F93A54702",
}

func (this *ToxPlugin) tox_login(ac *purple.Account) {
	this.stopch = make(chan struct{}, 0)
	this._toxopts = tox.NewToxOptions()
	this._toxopts.Tcp_port = uint16(rand.Uint32()%55536) + 10000
	this.load_account()

	// retry 50 times
	for port := 0; port < 50; port++ {
		this._toxopts.Tcp_port = uint16(rand.Uint32()%55536) + 10000
		this._tox = tox.NewTox(this._toxopts)
		if this._tox != nil {
			log.Println("TOXID:", this._tox.SelfGetAddress())
			break
		}
	}
	if this._tox == nil {
		log.Panicln("null")
	}

	for i := 0; i < len(bsnodes); i += 3 {
		port, _ := strconv.Atoi(bsnodes[i+1])
		ok1, err1 := this._tox.Bootstrap(bsnodes[i], uint16(port), bsnodes[i+2])
		ok2, err2 := this._tox.AddTcpRelay(bsnodes[i], uint16(port), bsnodes[i+2])
		if !ok1 || !ok2 || err1 != nil || err2 != nil {
			log.Println(ok1, ok2, err1, err2)
		}
	}

	// little state setup
	if true {
		conn := ac.GetConnection()
		conn.SetState(purple.CONNECTING)
	}

	this.setupSelfInfo(ac)
	this.setupCallbacks(ac)
	this.loadFriends(ac)

	if false {
		buddy := purple.NewBuddy(ac, "onlyyou-id", "onlyyou-nick")
		ac.AddBuddy(buddy)
		group := purple.NewGroup("GOTOX")
		buddy.BlistAdd(group)
	}

	go this.Iterate()
}

func (this *ToxPlugin) tox_close(gc *purple.Connection) {
	this.stopch <- struct{}{}
	this.save_account()
	this._tox.Kill()
	this._tox = nil
	this._toxopts = nil
}

func (this *ToxPlugin) tox_status_types() {
	log.Println("called")
}

////////
func (this *ToxPlugin) Iterate() {
	stopped := false
	tick := time.Tick(100 * time.Millisecond)
	for !stopped {
		select {
		case <-tick:
			this._tox.Iterate()
		case <-this.stopch:
			stopped = true
		}
	}
	log.Println("stopped", this._tox.SelfGetAddress())
}

var data_file = "/tmp/gotox.dat"

func (this *ToxPlugin) load_account() {
	data, err := tox.LoadSavedata(data_file)
	if err != nil {
		log.Println(err)
	} else {
		this._toxopts.Savedata_data = data
		this._toxopts.Savedata_type = tox.SAVEDATA_TYPE_TOX_SAVE
	}
}

func (this *ToxPlugin) save_account() {
	data := this._tox.GetSavedata()
	data64 := base64.StdEncoding.EncodeToString(data)

	err := this._tox.WriteSavedata(data_file)
	if err != nil {
		log.Println(len(data64))
		log.Println(err)
	}
}

func NewToxPlugin() *ToxPlugin {
	this := &ToxPlugin{}

	pi := purple.PluginInfo{
		Id:          "prpl-gotox",
		Name:        "GoTox",
		Version:     "1.0",
		Summary:     "it's summary",
		Description: "it's description",
		Author:      "it's gzleo",
		Homepage:    "https://fixlan.net/",

		Load:    this.load_tox,
		Unload:  this.unload_tox,
		Destroy: this.destroy_tox,
	}
	ppi := purple.PluginProtocolInfo{
		BlistIcon: this.tox_blist_icon,
		Login:     this.tox_login,
		Close:     this.tox_close,
	}
	this.p = purple.NewPlugin(&pi, &ppi, this.init_tox)

	return this
}

func init() {
	colog.Register()
	colog.SetFlags(log.LstdFlags | log.Lshortfile | colog.Flags())

	NewToxPlugin()
}

func main() {}