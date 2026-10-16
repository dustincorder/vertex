package main

import (
	"flag"
	"log"

	"vertex/internal/server"
)

func main() {
	addr := flag.String("listen", ":25565", "TCP address to listen on")
	flag.Parse()

	s := server.New(*addr)
	log.Printf("Vertex listening on %s (Minecraft Java %s, protocol %d)", *addr, server.MinecraftVersion, server.ProtocolVersion)
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
