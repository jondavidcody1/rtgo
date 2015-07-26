//    Title: conn.go
//    Author: Jon Cody
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <http://www.gnu.org/licenses/>.

package rtgo

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/websocket"
	"html"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Conn struct {
	Application *App
	Socket      *websocket.Conn
	Id          string
	Send        chan []byte
	Rooms       map[string]*Room
	Privilege   string
}

func (c *Conn) SendView(path string) {
	var (
		doc bytes.Buffer
		err error
	)
	route := c.Application.FindRoute(path)
	if _, ok := route["template"]; !ok {
		log.Println("No template for the specified path: ", path)
		return
	}
	collection := make([]interface{}, 0)
	if _, ok := route["table"]; ok {
		if _, ok := route["key"]; ok {
			if obj, err := c.Application.DB.GetObj(route["table"], route["key"]); err == nil {
				collection = append(collection, obj)
			}
		} else {
			collection, _ = c.Application.DB.GetAllObjs(route["table"])
		}
	}
	c.Application.Templates.ExecuteTemplate(&doc, route["template"], collection)
	data := map[string]string{
		"view":        route["template"],
		"template":    html.UnescapeString(doc.String()),
		"controllers": route["controllers"],
	}
	payload, err := json.Marshal(&data)
	if err != nil {
		log.Println("error encoding json: ", err)
		return
	}
	response := &Message{
		RoomLength:    len("root"),
		Room:          "root",
		EventLength:   len("response"),
		Event:         "response",
		DstLength:     len(c.Id),
		Dst:           c.Id,
		SrcLength:     len(c.Id),
		Src:           c.Id,
		PayloadLength: len(payload),
		Payload:       payload,
	}
	c.Send <- MessageToBytes(response)
}

func (c *Conn) HandleData(data []byte, msg *Message) error {
	switch msg.Event {
	case "join":
		c.Join(msg.Room)
	case "leave":
		c.Leave(msg.Room)
	case "request":
		c.SendView(string(msg.Payload))
	default:
		if msg.Dst != "" {
			if dst, ok := c.Rooms[msg.Room].Members[msg.Dst]; ok {
				dst.Send <- data
			}
		} else {
			c.Emit(data, msg)
		}
	}
	c.Application.Emitter.Emit(msg.Event, c, data, msg)
	return nil
}

func (c *Conn) ReadPump() {
	defer func() {
		for _, room := range c.Rooms {
			room.Leavechan <- c
		}
		c.Socket.Close()
	}()
	c.Socket.SetReadLimit(maxMessageSize)
	c.Socket.SetReadDeadline(time.Time{})
	c.Socket.SetPongHandler(func(string) error {
		c.Socket.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.Socket.ReadMessage()
		if err != nil {
			if err != io.EOF {
				log.Println("error parsing incoming message:", err)
			} else {
				for name, room := range c.Rooms {
					payload := &Message{
						RoomLength:    len(name),
						Room:          name,
						EventLength:   len("left"),
						Event:         "left",
						DstLength:     0,
						Dst:           "",
						SrcLength:     len(c.Id),
						Src:           c.Id,
						PayloadLength: len([]byte(c.Id)),
						Payload:       []byte(c.Id),
					}
					room.Emit(c, MessageToBytes(payload))
				}
			}
			break
		}
		if err := c.HandleData(data, BytesToMessage(data)); err != nil {
			log.Println(err)
		}
	}
}

func (c *Conn) Write(mt int, payload []byte) error {
	c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
	return c.Socket.WriteMessage(mt, payload)
}

func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Socket.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.Write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Write(websocket.BinaryMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (c *Conn) Join(name string) {
	var room *Room
	if _, ok := c.Application.RoomManager[name]; ok {
		room = c.Application.RoomManager[name]
	} else {
		room = c.Application.NewRoom(name)
	}
	c.Rooms[name] = room
	room.Join(c)
}

func (c *Conn) Leave(name string) {
	if room, ok := c.Application.RoomManager[name]; ok {
		delete(c.Rooms, room.Name)
		room.Leave(c)
	}
}

func (c *Conn) Emit(data []byte, msg *Message) {
	if room, ok := c.Application.RoomManager[msg.Room]; ok {
		room.Emit(c, data)
	}
}
