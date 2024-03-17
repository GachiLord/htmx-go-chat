# htmx-go-chat

Simple WebSocket chat written in golang using htmx and tailwind. React devs are not invited!

## features

- create a chat with the specified name
- join a chat by link(uuid)
- unused chats expire every 30 minutes if there are no users

## building

Build css:
`
cd static/resources
./prodCss.sh
`

Dev css:
`
cd static/resources
./devCss.sh
`

Run the server:
`
go run .
`
