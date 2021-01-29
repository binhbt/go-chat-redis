package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

// Peers maps a chat user to the websocket connection (pointer)
var Peers map[string]*websocket.Conn

//Subs map for subcribe channel
var Subs map[string]*redis.PubSub

func init() {
	Peers = map[string]*websocket.Conn{}
	Subs = map[string]*redis.PubSub{}
}

// ChatSession represents a connected/active chat user
type ChatSession struct {
	user  string
	group string
	peer  *websocket.Conn
}

// NewChatSession returns a new ChatSession
func NewChatSession(user string, group string, peer *websocket.Conn) *ChatSession {

	return &ChatSession{user: user, group: group, peer: peer}
}

const usernameHasBeenTaken = "username %s is already taken. please retry with a different name"
const retryMessage = "failed to connect. please try again"
const welcome = "Welcome %s!"
const welcomeJSON = `{"Content":"%s", "Channel":"%s", "From":"%s", "Type":"welcome"}`
const joined = "%s: has joined the chat!"
const joinedJSON = `{"Content":"%s", "Channel":"%s", "From":"%s", "Type":"joined"}`

// const chat = "%s: %s"
const left = "%s: has left the chat!"
const leftJSON = `{"Content":"%s", "Channel":"%s", "From":"%s", "Type":"left"}`
const peerkey = "%s:%s:%d"

// Start starts the chat by reading messages sent by the peer and broadcasting the to redis pub-sub channel
func (s *ChatSession) Start() {
	exist, err := CheckChannelExists(s.group)

	if exist {
		log.Println("Channel exist")
	} else {
		err := CreateChannel(s.group)
		if err != nil {
			log.Println("failed to add user to list of active chat users", s.user)
			return
		}
	}
	if _, ok := Subs[s.group]; ok {
		log.Println("Has subcribed -> Ignore")
	} else {
		startSubscriber(s.group)
		// Subs[s.group] = "OK"
	}

	usernameTaken, err := CheckUserExists(s.user)

	if err != nil {
		log.Println("unable to determine whether user exists -", s.user)
		s.notifyPeer(retryMessage)
		s.peer.Close()
		return
	}

	if usernameTaken {
		// msg := fmt.Sprintf(usernameHasBeenTaken, s.user)
		// s.peer.WriteMessage(websocket.TextMessage, []byte(msg))
		// s.peer.Close()
		// return
	} else {
		err = CreateUser(s.user)
		if err != nil {
			log.Println("failed to add user to list of active chat users", s.user)
			s.notifyPeer(retryMessage)
			s.peer.Close()
			return
		}
	}
	t := time.Now().UnixNano()
	key := fmt.Sprintf(peerkey, s.group, s.user, t)
	//user:time
	log.Println("peer key--", key)
	Peers[key] = s.peer
	ujoined, err := CheckUserInChannel(s.group, s.user)
	if !ujoined {
		UserJoinChannel(s.group, s.user)
	}
	welcomeMsg := fmt.Sprintf(welcomeJSON, fmt.Sprintf(welcome, s.user), s.group, s.user)
	// `{"Content":"` + fmt.Sprintf(welcome, s.user) + `", "Channel":"` + s.group + `", "From":"` + s.user + `", "Type":"welcome"}`
	// s.notifyPeer(fmt.Sprintf(welcome, s.user))
	s.notifyPeer(welcomeMsg)
	joinedMsg := fmt.Sprintf(joinedJSON, fmt.Sprintf(joined, s.user), s.group, s.user)
	// `{"Content":"` + fmt.Sprintf(joined, s.user) + `", "Channel":"` + s.group + `", "From":"` + s.user + `", "Type":"joined"}`
	SendToChannel(s.group, joinedMsg)

	/*
		this go-routine will exit when:
		(1) the user disconnects from chat manually
		(2) the app is closed
	*/
	go func() {
		log.Println("user joined", s.user)
		for {
			_, msg, err := s.peer.ReadMessage()
			if err != nil {
				_, ok := err.(*websocket.CloseError)
				if ok {
					log.Println("connection closed by user")
					s.disconnect()
				}
				return
			}
			msgChat := getChatMessage(string(msg))
			msgChat.From = s.user
			msgChat.Channel = s.group
			msgChat.Type = "text"
			msgChat.TimeStamp = time.Now().Unix()
			data, _ := json.Marshal(msgChat)
			SendToChannel(s.group, string(data))
		}
	}()
}

func (s *ChatSession) notifyPeer(msg string) {
	err := s.peer.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Println("failed to write message", err)
	}
}

// Invoked when the user disconnects (websocket connection is closed). It performs cleanup activities
func (s *ChatSession) disconnect() {
	//remove user from SET
	RemoveUser(s.user)
	leftMsg := fmt.Sprintf(leftJSON, fmt.Sprintf(left, s.user), s.group, s.user)
	// `{"Content":"` + fmt.Sprintf(left, s.user) + `", "Channel":"` + s.group + `", "From":"` + s.user + `", "Type":"left"}`
	//notify other users that this user has left
	SendToChannel(s.group, leftMsg)

	//close websocket
	s.peer.Close()

	//remove from Peers
	delete(Peers, s.user)
}
