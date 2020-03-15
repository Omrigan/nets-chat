package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"

	bolt "go.etcd.io/bbolt"
)

type ChatConn struct {
	conn   net.Conn
	server *Server
	scan   *bufio.Scanner
	login  string
}

func (c *ChatConn) Close() {
	c.conn.Close()
	delete(c.server.conns, c)
}

func (s *ChatConn) Read() string {
	res := s.scan.Scan()
	if !res {
		fmt.Println("Err!")
		return ""
	}
	str := s.scan.Text()
	fmt.Println("Receiving:", str)
	return str
}

func (s *ChatConn) Write(message string) {
	str := fmt.Sprintf("%v\r\n", message)
	fmt.Println("Sending:", str[:len(str)-2])
	io.WriteString(s.conn, str)
}

type Server struct {
	conns map[*ChatConn]bool
	db    *bolt.DB
}

func NewServer() *Server {
	return &Server{
		conns: make(map[*ChatConn]bool),
	}
}

func (s *Server) Broadcast(who *ChatConn, what string) {
	for user := range s.conns {
		user.Write(fmt.Sprintf("%s: %s", who.login, what))
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	c := &ChatConn{
		conn:   conn,
		scan:   bufio.NewScanner(conn),
		server: s,
		login:  "",
	}
	s.conns[c] = true
	defer c.Close()

	for {
		msg := c.Read()
		if strings.HasPrefix(msg, "QUIT") {
			c.Write("Bye")
			return
		} else if strings.HasPrefix(msg, "LOGOUT") {
			c.login = ""
			c.Write("Logged out")
			continue
		} else if strings.HasPrefix(msg, "LISTALL") {
			if c.login == "admin" {
				s.db.View(func(tx *bolt.Tx) error {
					result := make([]string, 0)
					// Assume bucket exists and has keys
					b := tx.Bucket([]byte("users"))

					cur := b.Cursor()
					for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
						result = append(result, string(k))
					}
					c.Write(fmt.Sprintf("All users: %s", strings.Join(result, " ")))
					return nil
				})
			} else {
				c.Write("You are not admin")
			}
			continue
		} else if strings.HasPrefix(msg, "LISTONLINE") {
			if c.login == "admin" {
				result := make([]string, 0)

				for k := range s.conns {
					result = append(result, k.login)
				}
				c.Write(fmt.Sprintf("All users: %s", strings.Join(result, " ")))
			} else {
				c.Write("You are not admin")
			}
			continue
		} else if strings.HasPrefix(msg, "LOGIN") {
			tokens := strings.SplitN(msg, " ", 3)
			if len(tokens) < 3 {
				c.Write("Login and password is required")
				continue
			}

			s.db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("users"))
				v := b.Get([]byte(tokens[1]))
				if v == nil {
					c.Write("No such account")
					return nil
				}
				truePass := string(v)
				if truePass != tokens[2] {
					c.Write("Wrong password")
					return nil
				}
				c.login = tokens[2]
				c.Write("Logged in")
				return nil
			})
			continue
		} else if strings.HasPrefix(msg, "REGISTER") {
			tokens := strings.SplitN(msg, " ", 3)
			if len(tokens) < 3 {
				c.Write("Login and password is required")
				continue
			}

			s.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("users"))
				v := b.Get([]byte(tokens[1]))
				if v != nil {
					c.Write("User already exists")
					return nil
				}

				err := b.Put([]byte(tokens[1]), []byte(tokens[2]))
				if err != nil {
					return err
				}
				c.login = tokens[1]
				c.Write("Registered")
				return nil
			})
			continue
		}

		if c.login == "" {
			c.Write("Need to login")
			continue
		}
		s.Broadcast(c, msg)
	}

}

func main() {
	listen := flag.String("listen", ":8080", "listen on")

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Listenting")

	server := NewServer()
	server.db, err = bolt.Open("bolt.db", 0666, nil)
	if err != nil {
		panic(err.Error())
	}
	server.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			panic(err.Error())
		}
		return nil
	})
	defer server.db.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err.Error())
		}
		go server.handleConnection(conn)
	}
}
