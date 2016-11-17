/*
  all implemention code except must code.
*/
package main

import (
	"fmt"
	"log"
	"strings"

	"yobot/purple"

	"github.com/kitech/go-toxcore"
)

func (this *ToxPlugin) setupSelfInfo(ac *purple.Account) {
	if len(ac.GetAlias()) > 0 {
		this._tox.SelfSetName(ac.GetAlias())
	} else {
		this._tox.SelfSetName(ac.GetUserName())
	}
	this._tox.SelfSetStatusMessage("It's from gotox-prpl, hoho.")

	conn := ac.GetConnection()
	purple.RequestYesNo(this, conn, func(ud interface{}) {
		log.Println("request yes")
	}, func(ud interface{}) {
		log.Println("request no")
	})
}

func (this *ToxPlugin) setupCallbacks(ac *purple.Account) {
	conn := ac.GetConnection()
	this._tox.CallbackSelfConnectionStatus(func(t *tox.Tox, status uint32, d interface{}) {
		if status > tox.CONNECTION_NONE {
			conn.ConnSetState(purple.CONNECTED) // 设置为已连接状态，则好友会显示。
		} else {
			conn.ConnSetState(purple.DISCONNECTED)
		}
	}, ac)

	this._tox.CallbackFriendRequest(func(t *tox.Tox, pubkey, msg string, d interface{}) {
		log.Println("hehhe", pubkey, msg)
		// TODO notice UI and then make desision
		this._tox.FriendAddNorequest(pubkey)
		this.save_account(conn)
	}, ac)

	this._tox.CallbackFriendConnectionStatus(func(t *tox.Tox, friendNumber uint32, status uint32, d interface{}) {
		log.Println(friendNumber, status)
		pubkey, _ := t.FriendGetPublicKey(friendNumber)
		name, _ := t.FriendGetName(friendNumber)

		buddy := this.findBuddyEx(ac, pubkey)
		if buddy == nil {
			log.Println("can not find buddy:", name, pubkey)
		} else {
			switch status {
			case tox.CONNECTION_NONE:
				purple.PrplGotUserStatus(ac, buddy.GetName(), STATUS_OFFLINE_STR)
			case tox.CONNECTION_TCP:
				purple.PrplGotUserStatus(ac, buddy.GetName(), STATUS_BUSY_STR)
			case tox.CONNECTION_UDP:
				purple.PrplGotUserStatus(ac, buddy.GetName(), STATUS_ONLINE_STR)
			}
		}
	}, ac)

	this._tox.CallbackFriendMessage(func(t *tox.Tox, friendNumber uint32, msg string, d interface{}) {
		log.Println(friendNumber, msg, purple.MyTid2())
		conn := ac.GetConnection()
		pubkey, err := t.FriendGetPublicKey(friendNumber)
		if err != nil {
			log.Println(err)
		} else {
			buddy := this.findBuddyEx(ac, pubkey)
			if buddy == nil {
				log.Println("wtf", friendNumber, pubkey)
			} else {
				conn.ServGotIM(buddy.GetName(), msg, purple.MESSAGE_RECV)
			}
		}
	}, ac)

	this._tox.CallbackFriendName(func(t *tox.Tox, friendNumber uint32, name string, d interface{}) {
		// log.Println(friendNumber, name)
		pubkey, err := t.FriendGetPublicKey(friendNumber)
		if err != nil {
			log.Println(err, pubkey)
		} else {
			buddy := this.findBuddyEx(ac, pubkey)
			if buddy != nil {
				if buddy.GetAliasOnly() != name {
					buddy.SetAlias(name)
				}
			} else {
				log.Println("wtf", friendNumber, name, pubkey)
				// buddy := purple.NewBuddy(ac, pubkey, name)
				// ac.AddBuddy(buddy)
				// buddy.BlistAdd(nil)
			}
		}
	}, nil)

	this._tox.CallbackGroupNameListChange(func(t *tox.Tox, groupNumber int,
		peerNumber int, change uint8, d interface{}) {
		log.Println(groupNumber, peerNumber, change)
		conn := ac.GetConnection()
		conv := conn.ConnFindChat(groupNumber)
		switch change {
		case tox.CHAT_CHANGE_PEER_ADD:
		case tox.CHAT_CHANGE_PEER_DEL:
		case tox.CHAT_CHANGE_PEER_NAME:
		}
		// TODO member list diff and clean, so it is member list sync
		this.UpdateMembers(groupNumber, conv)
	}, ac)

	this._tox.CallbackGroupMessage(func(t *tox.Tox, groupNumber int,
		peerNumber int, message string, d interface{}) {
		conn := ac.GetConnection()
		pubkey, err := t.GroupPeerPubkey(groupNumber, peerNumber)
		peerName, err := t.GroupPeerName(groupNumber, peerNumber)
		if err != nil {
			log.Println(err)
		} else {
			if false {
				conn.ServGotChatIn(groupNumber, pubkey, purple.MESSAGE_RECV, message)
			}
			conn.ServGotChatIn(groupNumber, peerName, purple.MESSAGE_RECV, message)
		}
	}, ac)
}

func (this *ToxPlugin) loadFriends(ac *purple.Account) {
	fns := this._tox.SelfGetFriendList()
	if fns == nil || len(fns) == 0 {
		log.Println("oh, you have 0 friends")
	}
	buddies := ac.FindBuddies("")
	for _, fn := range fns {
		name, err := this._tox.FriendGetName(fn)
		pubkey, err := this._tox.FriendGetPublicKey(fn)
		if err != nil {
			log.Println(err)
		}

		buddy := ac.FindBuddy(pubkey)
		if buddy == nil {
			found := false
			for _, _buddy := range buddies {
				if strings.HasPrefix(_buddy.GetName(), pubkey) {
					found = true
					break
				}
			}
			if !found {
				buddy = purple.NewBuddy(ac, pubkey, name)
				ac.AddBuddy(buddy)
				buddy.BlistAdd(nil)
			}
		} else {
			if buddy.GetAliasOnly() != name {
				buddy.SetAlias(name)
			}
		}
		// purple.PrplGotUserStatus(ac, buddy.GetName(), STATUS_ONLINE_STR)
		log.Println("adding...", name, pubkey, purple.MyTid2())
	}
}

// 因为存储的name可能是friendId，也可能是pubkey。
func (this *ToxPlugin) findBuddyEx(ac *purple.Account, pubkeyOrFriendID string) *purple.Buddy {
	name := pubkeyOrFriendID
	buddy := ac.FindBuddy(name)
	if buddy == nil {
		buddies := ac.FindBuddies("")
		for _, buddy_ := range buddies {
			if strings.HasPrefix(buddy_.GetName(), name) {
				buddy = buddy_
				break
			}
		}
	}
	return buddy
}

// optional callbacks
func (this *ToxPlugin) ChatInfo(gc *purple.Connection) []string {
	log.Println(gc)
	infos := []string{"ToxChannel"}
	return infos
}

func (this *ToxPlugin) ChatInfoDefaults(gc *purple.Connection, chatName string) map[string]string {
	log.Println(gc)
	return nil
}

func (this *ToxPlugin) SendIM(gc *purple.Connection, who string, msg string) int {
	log.Println(gc, who, msg)
	friendNumber, _ := this._tox.FriendByPublicKey(who)
	len, err := this._tox.FriendSendMessage(friendNumber, msg)
	if err != nil {
		log.Println(err, len)
		return -1
	}
	return int(len)
}

func (this *ToxPlugin) JoinChat(gc *purple.Connection, comp *purple.GHashTable) {
	groupNumber, err := this._tox.AddGroupChat()
	if err != nil {
		log.Println(err)
	}
	title := comp.Lookup("ToxChannel")
	this._tox.GroupSetTitle(groupNumber, title)
	comp.Insert("GroupNumber", fmt.Sprintf("%d", groupNumber))
	conv := gc.ServGotJoinedChat(groupNumber, comp.Lookup("ToxChannel"))
	if conv != nil {
	}
	this.UpdateMembers(groupNumber, conv)
}
func (this *ToxPlugin) RejectChat(gc *purple.Connection, comp *purple.GHashTable) {
	log.Println("herhere")
	log.Println(comp.ToMap())
}
func (this *ToxPlugin) GetChatName(comp *purple.GHashTable) string {
	log.Println("herhere")
	log.Println(comp.ToMap())
	return ""
}
func (this *ToxPlugin) ChatInvite(gc *purple.Connection, id int, message string, who string) {
	log.Println("herhere")
	log.Println("herhere", id, message, who)
	friendNumber, err := this._tox.FriendByPublicKey(who)
	if err != nil {
		log.Println(err)
	}
	rc, err := this._tox.InviteFriend(friendNumber, id)
	if err != nil {
		log.Println(rc, err)
	}
}
func (this *ToxPlugin) ChatLeave(gc *purple.Connection, id int) {
	log.Println("herhere")
}
func (this *ToxPlugin) ChatWhisper(gc *purple.Connection, id int, who string, message string) {
	log.Println("herhere")
}
func (this *ToxPlugin) ChatSend(gc *purple.Connection, id int, message string, flags int) int {
	log.Println("herhere")
	n, err := this._tox.GroupMessageSend(id, message)
	if err != nil {
		log.Println(err)
	}
	if n == -1 {
		// log.Println("still send ok, wtf")
	}
	log.Println(n, id, message, flags)
	return len(message)
}

func (this *ToxPlugin) RoomlistGetList(gc *purple.Connection) {
	log.Println("herere")
}

func (this *ToxPlugin) AddBuddyWithInvite(gc *purple.Connection,
	buddy *purple.Buddy, group *purple.Group, message string) {
	log.Println(buddy, group, message)
	friendId := buddy.GetName()
	if len(message) == 0 {
		message = fmt.Sprintf("This is %s", this._tox.SelfGetName())
	}
	friendNumber, err := this._tox.FriendAdd(friendId, message)
	if err != nil {
		log.Println(err, friendNumber)
	} else {
		// gc.ConnGetAccount().AddBuddy(buddy)
		// buddy.BlistAdd(nil)
		buddy := gc.ConnGetAccount().FindBuddy(friendId)
		log.Println(buddy)
	}
}

func (this *ToxPlugin) RemoveBuddy(gc *purple.Connection, buddy *purple.Buddy, group *purple.Group) {
	friendId := buddy.GetName()
	friendNumber, err := this._tox.FriendByPublicKey(friendId)
	if err != nil {
		log.Println(err, friendNumber)
	} else {
		_, err = this._tox.FriendDelete(friendNumber)
		if err != nil {
			log.Println(err)
		}
	}
}

// utils
func (this *ToxPlugin) UpdateMembers(groupNumber int, conv *purple.Conversation) {
	chat := conv.GetChatData()
	// TODO member list diff and clean, so it is member list sync
	t := this._tox
	plst := chat.GetUsers()
	tlst := t.GroupGetNames(groupNumber)
	klst := t.GroupGetPeerPubkeys(groupNumber)
	mlst := t.GroupGetPeers(groupNumber)
	peerCount := t.GroupNumberPeers(groupNumber)
	if len(tlst) != peerCount {
		log.Println("wtf")
	}

	if true {
		log.Println("need sync names...")
		log.Println("purple list:", plst)
		log.Println("tox list:", tlst)
		log.Println("pubkey list:", klst)
		log.Println("peer list:", mlst)

		// remove not existed
		for _, pname := range plst {
			found := false
			for _, tname := range tlst {
				if tname == pname {
					found = true
				}
			}
			if found == false {
				chat.RemoveUser(pname) // should already destroy the ConvChatBuddy here
				cbbuddy := chat.FindBuddy(pname)
				cbbuddy.Destroy()
			}
		}

		// add new
		for peerNumber, pubkey := range mlst {
			found := false
			peerName, err := t.GroupPeerName(groupNumber, peerNumber)
			if err != nil {
			}
			for _, pname := range plst {
				if pname == peerName {
					found = true
				}
			}
			if found == false {
				isours := t.GroupPeerNumberIsOurs(groupNumber, peerNumber)
				if isours == true {
				}
				if true {
					chat.AddUser(peerName)
					cbbudy := chat.FindBuddy(peerName)
					cbbudy.SetAlias(pubkey)
				}
			}
		}
	}

}
