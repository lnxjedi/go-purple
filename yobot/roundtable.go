package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	// "github.com/thoj/go-ircevent"
)

const (
	EVT_NONE                = "none"
	EVT_CONNECTED           = "connected"
	EVT_DISCONNECTED        = "disconnected"
	EVT_FRIEND_CONNECTED    = "friend_connected"
	EVT_FRIEND_DISCONNECTED = "friend_disconnected"
	EVT_FRIEND_MESSAGE      = "friend_message"
	EVT_JOIN_GROUP          = "join_group"
	EVT_GROUP_MESSAGE       = "group_message"
)

const MAX_BUS_QUEUE_LEN = 123

type Event struct {
	Proto string
	EType string
	Chan  string
	Args  []interface{}
	// RawEvent interface{}
	Be Backend
}

func NewEvent(proto string, etype string, ch string, args ...interface{}) *Event {
	this := &Event{}
	this.Proto = proto
	this.EType = etype
	this.Chan = ch
	this.Args = args
	return this
}

type RoundTable struct {
	ctx *Context
}

func NewRoundTable() *RoundTable {
	this := &RoundTable{}
	this.ctx = ctx
	return this
}

func (this *RoundTable) run() {
	go this.handleEvent()
	select {}
}

func (this *RoundTable) handleEvent() {
	for ie := range this.ctx.busch {
		// log.Println(ie)
		switch ie.Proto {
		case PROTO_IRC:
			// this.handleEventIrc(ie.RawEvent.(*irc.Event))
			this.handleEventIrc(ie)
		case PROTO_TOX:
			this.handleEventTox(ie)
		}
	}
}

func (this *RoundTable) handleEventTox(e *Event) {
	log.Printf("%+v", e)
	switch e.EType {
	case EVT_GROUP_MESSAGE:
		if this.processGroupCmd(e.Args[0].(string), e.Args[1].(int), e.Args[2].(int)) {
			break
		}

		peerName, err := this.ctx.toxagt._tox.GroupPeerName(e.Args[1].(int), e.Args[2].(int))
		if err != nil {
			log.Println(err)
		}
		var fromUser = fmt.Sprintf("%s[t]", peerName)
		groupTitle := e.Chan
		message := e.Args[0].(string)
		var chname string = groupTitle
		if key, found := chmap.GetKey(groupTitle); found {
			// forward message to...
			chname = key.(string)
		}

		// root user
		if !this.ctx.acpool.has(ircname) {
			log.Println("wtf, try fix")
			rac := this.ctx.acpool.add(ircname)
			rac.conque <- e
		} else {
			rac := this.ctx.acpool.get(ircname)
			be := rac.becon.(*IrcBackend)
			if !be.isconnected() {
				log.Println("Oh, maybe unexpected")
			}
			be.join(chname)

			// agent user
			var ac *Account
			if !this.ctx.acpool.has(fromUser) {
				ac = this.ctx.acpool.add(fromUser)
				ac.conque <- e
			} else {
				ac := this.ctx.acpool.get(fromUser)
				be := ac.becon.(*IrcBackend)
				if !be.isconnected() {
					log.Println("Oh, connection broken, ", chname)
					err := be.reconnect()
					if err != nil {
						log.Println(err)
					}
				}
				be.join(chname)
				messages := strings.Split(message, "\n") // fix multiple line message
				for _, m := range messages {
					be.sendGroupMessage(m, chname)
				}
			}
		}

	case EVT_JOIN_GROUP:

		if !this.ctx.acpool.has(ircname) {
			ac := this.ctx.acpool.add(ircname)
			ac.conque <- e
		} else {
			groupTitle := e.Chan
			chname := groupTitle
			if key, found := chmap.GetKey(groupTitle); found {
				// forward message to...
				chname = key.(string)
			}
			ac := this.ctx.acpool.get(ircname)
			be := ac.becon.(*IrcBackend)
			be.join(chname)
		}

	case EVT_FRIEND_MESSAGE:
		friendNumber := e.Args[1].(uint32)
		cmd := e.Args[0].(string)
		segs := strings.Split(cmd, " ")

		switch segs[0] {
		case "info": // show friends count, groups count and group list info
			this.processInfoCmd(friendNumber)
		case "invite":
			if len(segs) > 1 {
				this.processInviteCmd(segs[1:], friendNumber)
			} else {
				this.ctx.toxagt._tox.FriendSendMessage(friendNumber, "invite what?")
			}
		case "id":
			this.ctx.toxagt._tox.FriendSendMessage(friendNumber,
				this.ctx.toxagt._tox.SelfGetAddress())
		case "help":
			this.ctx.toxagt._tox.FriendSendMessage(friendNumber, cmdhelp)
		default:
			this.ctx.toxagt._tox.FriendSendMessage(friendNumber, invalidcmd)
		}
	}
}

func (this *RoundTable) processInviteCmd(channels []string, friendNumber uint32) {
	t := this.ctx.toxagt._tox

	// for groupbot groups

	// for irc groups
	for _, chname := range channels {
		if chname == "" {
			this.ctx.toxagt._tox.FriendSendMessage(friendNumber, invalidcmd)
			continue
		}

		groupNumbers := this.ctx.toxagt._tox.GetChatList()
		found := false
		var groupNumber int
		for _, gn := range groupNumbers {
			groupTitle, err := this.ctx.toxagt._tox.GroupGetTitle(int(gn))
			if err != nil {
				log.Println("wtf")
			} else {
				if groupTitle == chname {
					found = true
					groupNumber = int(gn)
				}
			}
		}
		if found {
			log.Println("already exists:", chname)
			_, err := t.InviteFriend(friendNumber, groupNumber)
			if err != nil {
				log.Println("wtf")
			}
			continue
		}

		_, err := strconv.Atoi(chname)
		if err == nil {
			// for groupbot groups
			friendNumber, err := this.ctx.toxagt._tox.FriendByPublicKey(groupbot)
			if err != nil {
				log.Println(err)
			}
			invcmd := fmt.Sprintf("invite %s", chname)
			ret, err := this.ctx.toxagt._tox.FriendSendMessage(friendNumber, invcmd)
			if err != nil {
				log.Println(err, ret)
			}
			go func() {
			}()
		} else {
			// for irc groups
			groupNumber, err := this.ctx.toxagt._tox.AddGroupChat()
			if err != nil {
				log.Println("wtf")
			} else {
				_, err := t.GroupSetTitle(groupNumber, chname)
				_, err = t.InviteFriend(friendNumber, groupNumber)
				if err != nil {
					log.Println("wtf")
				}
			}
		}
		// ac := this.ctx.acpool.get(chname)
	}
}

var myonlineTime = time.Now()

func (this *RoundTable) processInfoCmd(friendNumber uint32) {
	info := ""

	info += fmt.Sprintf("Uptime: %s\n\n", time.Now().Sub(myonlineTime).String())
	info += fmt.Sprintf("Friends: %d (105 online)\n\n",
		this.ctx.toxagt._tox.SelfGetFriendListSize())

	groupNumbers := this.ctx.toxagt._tox.GetChatList()
	for _, groupNumber := range groupNumbers {
		groupTitle, err := this.ctx.toxagt._tox.GroupGetTitle(int(groupNumber))
		if err != nil {
			log.Println(err)
		}
		peerCount := this.ctx.toxagt._tox.GroupNumberPeers(int(groupNumber))
		info += fmt.Sprintf("Group %d | Text | peers: %d | Title: %s\n\n",
			groupNumber, peerCount, groupTitle)
	}

	this.ctx.toxagt._tox.FriendSendMessage(friendNumber, info)
}

// 如果是cmd则返回true
func (this *RoundTable) processGroupCmd(msg string, groupNumber, peerNumber int) bool {
	groupTitle, err := this.ctx.toxagt._tox.GroupGetTitle(groupNumber)
	if err != nil {
		log.Println(err)
	}
	segs := strings.Split(msg, " ")
	if len(segs) == 1 {
		switch segs[0] {
		case "names":
		case "nc": // name count of peer irc
			ac := this.ctx.acpool.get(ircname)
			if ac == nil {
				log.Println("not connected to ", groupTitle)
				this.ctx.toxagt._tox.GroupMessageSend(groupNumber, "not connected to irc:"+groupTitle)
			} else {
				ircon := ac.becon.(*IrcBackend)
				ircon.ircon.SendRaw("/users")
			}
			return true
		case "ping":
			ac := this.ctx.acpool.get(ircname)
			if ac == nil {
				log.Println("not connected to ", groupTitle)
				this.ctx.toxagt._tox.GroupMessageSend(groupNumber, "not connected to irc:"+groupTitle)
			} else {
				ircon := ac.becon.(*IrcBackend)
				ircon.ircon.SendRaw(fmt.Sprintf("/whois %s", ircname))
			}
			return true
		case "raw":
			this.ctx.toxagt._tox.GroupMessageSend(groupNumber, "raw what?")
			return true
		}
	} else if len(segs) > 1 {
		switch segs[0] {
		case "raw":
			ac := this.ctx.acpool.get(ircname)
			if ac == nil {
				log.Println("not connected to ", groupTitle)
				this.ctx.toxagt._tox.GroupMessageSend(groupNumber, "not connected to irc:"+groupTitle)
			} else {
				ircon := ac.becon.(*IrcBackend)
				ircon.ircon.SendRaw(fmt.Sprintf("%s", strings.Join(segs[1:], " ")))
			}
			return true
		}
	}
	return false
}

func (this *RoundTable) handleEventIrc(e *Event) {
	be := e.Be.(*IrcBackend)

	switch e.EType {
	case EVT_CONNECTED: // MOTD end
		// ircon.Join("#tox-cn123")
		ac := this.ctx.acpool.get(be.getName())
		for len(ac.conque) > 0 {
			e := <-ac.conque
			this.ctx.busch <- e
		}

	case EVT_GROUP_MESSAGE:
		nick := e.Args[0].(string)
		// 检查是否是root用户连接
		if be.getName() != ircname {
			break // forward message only by root user
		}
		// 检查来源是否是我们自己的连接发的消息
		if _, ok := this.ctx.acpool.acs[e.Args[0].(string)]; ok {
			break
		}

		chname := e.Args[1].(string)
		message := e.Args[2].(string)
		message = fmt.Sprintf("[%s] %s", nick, message)

		if val, found := chmap.Get(chname); found {
			chname = val.(string)
		}

		// TODO maybe multiple result
		groupNumber := this.ctx.toxagt.getToxGroupByName(chname)
		if groupNumber == -1 {
			log.Println("group not exists:", chname)
		} else {
			_, err := this.ctx.toxagt._tox.GroupMessageSend(groupNumber, message)
			if err != nil {
				// should be 1
				pno := this.ctx.toxagt._tox.GroupNumberPeers(groupNumber)
				log.Println(err, chname, groupNumber, message, pno)
			}
		}

	case EVT_JOIN_GROUP:
	case EVT_DISCONNECTED:
		// close reconnect/ by Excess Flood/
		this.ctx.acpool.remove(ircname)
	default:
		log.Println("unknown evt:", e.EType)

	}

}
