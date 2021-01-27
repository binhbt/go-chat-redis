package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

//MessageData nessage data struct
type MessageData struct {
	From    string
	Channel string
	Content string
	Type    string
}

var client *redis.Client
var redisHost string
var redisPassword string

// var sub *redis.PubSub

func init() {

	redisHost = os.Getenv("REDIS_HOST")
	if redisHost == "" {
		log.Fatal("missing REDIS_HOST env var")
	}

	// redisPassword = os.Getenv("REDIS_PASSWORD")
	// if redisPassword == "" {
	// 	log.Fatal("missing REDIS_PASSWORD env var")
	// }

	log.Println("connecting to Redis...")
	client = redis.NewClient(&redis.Options{Addr: redisHost, Password: "", DB: 0})

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatal("failed to connect to redis", err)
	}
	log.Println("connected to redis", redisHost)
	// startSubscriber()
}
func getChatMessage(payload string) MessageData {
	log.Println("Message ...", payload)
	var msg MessageData
	json.Unmarshal([]byte(payload), &msg)
	fmt.Printf("Message : %+v", msg)
	fmt.Printf("From: %s, Message: %s", msg.From, msg.Channel)
	return msg
}
func startSubscriber(ChannelName string) {
	/*
		this goroutine exits when the application shuts down. When the pusub connection is closed,
		the channel range loop terminates, hence terminating the goroutine
	*/
	go func() {
		log.Println("starting subscriber...", ChannelName)
		sub := client.Subscribe(ChannelName)
		Subs[ChannelName] = sub
		messages := sub.Channel()
		for message := range messages {
			log.Println("Receive from...", ChannelName)

			msg := getChatMessage(message.Payload)
			from := msg.From
			log.Println("From...", from)
			// from := strings.Split(message.Payload, ":")[0]
			//send to all websocket sessions/peers
			for userinstance, peer := range Peers {
				//group:user:time
				group := strings.Split(userinstance, ":")[0]
				user := strings.Split(userinstance, ":")[1]
				if from != user { //don't recieve your own messages
					log.Println("User...", user)
					joined, _ := CheckUserInChannel(ChannelName, user)
					log.Println("Check in channel...", ChannelName, joined)
					//If don't check group it will send to all user group and you just open one connection
					if joined && group == ChannelName {
						peer.WriteMessage(websocket.TextMessage, []byte(message.Payload))
					}
				}
			}
		}
	}()
}

// SendToChannel pusblishes on a redis pubsub channel
func SendToChannel(channel string, msg string) {
	err := client.Publish(channel, msg).Err()
	if err != nil {
		log.Println("could not publish to channel", err)
	}
}

const channels = "chat-channels"
const users = "chat-users"

// CheckUserInChannel checks whether the user exists in the SET of active chat users
func CheckUserInChannel(channel string, user string) (bool, error) {
	joined, err := client.SIsMember(channel, user).Result()
	if err != nil {
		return false, err
	}
	return joined, nil
}

// CheckUserExists checks whether the user exists in the SET of active chat users
func CheckUserExists(user string) (bool, error) {
	usernameTaken, err := client.SIsMember(users, user).Result()
	if err != nil {
		return false, err
	}
	return usernameTaken, nil
}

// CheckChannelExists checks whether the user exists in the SET of active chat channels
func CheckChannelExists(channel string) (bool, error) {
	usernameTaken, err := client.SIsMember(channels, channel).Result()
	if err != nil {
		return false, err
	}
	return usernameTaken, nil
}

// CreateChannel creates a new channel in the SET of active chat channels
func CreateChannel(channel string) error {
	err := client.SAdd(channels, channel).Err()
	if err != nil {
		return err
	}
	return nil
}

// CreateUser creates a new user in the SET of active chat users
func CreateUser(user string) error {
	err := client.SAdd(users, user).Err()
	if err != nil {
		return err
	}
	return nil
}

// UserJoinChannel SET users in group
func UserJoinChannel(channel string, user string) error {
	err := client.SAdd(channel, user).Err()
	if err != nil {
		return err
	}
	return nil
}

// RemoveUser removes a user from the SET of active chat users
func RemoveUser(user string) {
	err := client.SRem(users, user).Err()
	if err != nil {
		log.Println("failed to remove user:", user)
		return
	}
	log.Println("removed user from redis:", user)
}

// RemoveChannel removes a user from the SET of active chat channels
func RemoveChannel(channel string) {
	err := client.SRem(channels, channel).Err()
	if err != nil {
		log.Println("failed to remove user:", channel)
		return
	}
	log.Println("removed user from redis:", channel)
}

// Cleanup is invoked when the app is shutdown - disconnects websocket peers, closes pusb-sub and redis client connection
func Cleanup() {
	for user, peer := range Peers {
		client.SRem(users, user)
		peer.Close()
	}
	log.Println("cleaned up users and sessions...")
	for channel, sub := range Subs {
		err := sub.Unsubscribe(channel)
		if err != nil {
			log.Println("failed to unsubscribe redis channel subscription:", err)
		}
		err = sub.Close()
		if err != nil {
			log.Println("failed to close redis channel subscription:", err)
		}
	}

	err := client.Close()
	if err != nil {
		log.Println("failed to close redis connection: ", err)
		return
	}
}
