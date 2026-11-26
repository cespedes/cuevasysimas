// Command issuer registers the clients of the catalogue, and makes the keys they sign with.
//
//	issuer keygen
//	    makes an Ed25519 key pair and prints it (base64): the private key is the client's, which keeps it and
//	    signs its assertions with it; the public one is what is registered here.
//	issuer register -issuer katiuskas-intranet -name "Club Katiuskas intranet" -public-key <base64>
//	    registers a client, or changes its key (DATABASE_URL says which database).
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/platform/db"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "keygen":
		keygen()
	case "register":
		register(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: issuer keygen | issuer register -issuer NAME -name DISPLAY_NAME -public-key BASE64")
	os.Exit(2)
}

func keygen() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate the key:", err)
		os.Exit(1)
	}
	fmt.Println("public-key: ", base64.StdEncoding.EncodeToString(pub))
	fmt.Println("private-key:", base64.StdEncoding.EncodeToString(priv))
}

func register(args []string) {
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	issuer := fs.String("issuer", "", "the name of the client as it appears in its assertions (iss)")
	name := fs.String("name", "", "how its service account is shown")
	publicKey := fs.String("public-key", "", "its Ed25519 public key, in base64")
	_ = fs.Parse(args)
	key, err := base64.StdEncoding.DecodeString(*publicKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "the public key is not base64:", err)
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect to the database:", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := accounts.NewService(accounts.NewRepository(pool)).RegisterIssuer(ctx, *issuer, *name, key); err != nil {
		fmt.Fprintln(os.Stderr, "register the issuer:", err)
		os.Exit(1)
	}
	fmt.Println("registered", *issuer)
}
