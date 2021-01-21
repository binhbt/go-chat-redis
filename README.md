#Run  
`docker-compose up --build`  
#Test  
//terminal 1 (user "foo")  
wscat -c ws://localhost:8080/chat/foo  
//terminal 2 (user "bar")  
wscat -c ws://localhost:8080/chat/bar  

https://itnext.io/lets-learn-how-to-to-build-a-chat-application-with-redis-websocket-and-go-7995b5c7b5e5  
