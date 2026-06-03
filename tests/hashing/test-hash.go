package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"hash/maphash"
	"log"
	"strconv"
	"time"
)

// compare performance of maphash-based cache key vs hmac-sha256 for caching bcrypt verification results in BasicAuthProvider.
// This is not a benchmark of the authentication itself, just the cache key generation.

type user struct {
	username string
	password string
}

func generateData(n int) []user {
	users := make([]user, n)
	for i := 0; i < n; i++ {
		// Generate user data (e.g., "user0:password0", "user1:password1", ...)
		u := user{
			username: "user" + strconv.Itoa(i),
			password: "password" + strconv.Itoa(i),
		}
		users[i] = u
	}
	return users
}

func run_maphash(user, pwd string) {
	var h maphash.Hash
	h.WriteString(user)
	h.WriteByte(':')
	h.WriteString(pwd)
	_ = h.Sum64() // Use the result to prevent compiler optimizations
}

func run_hmac_sha256(user, pwd string) {
	// actual hmac-shae256 hashing
	h := hmac.New(sha256.New, []byte("secret-key"))
	h.Write([]byte(user + ":" + pwd))
	_ = h.Sum(nil) // Use the result to prevent compiler optimizations
}

func test(f func(user, pwd string), data []user, repeats int, name string) {
	start := time.Now()
	log.Default().Printf("Running %d iterations of %s...", repeats*len(data), name)
	for i := 0; i < repeats; i++ {
		for _, u := range data {
			f(u.username, u.password)
		}
	}
	elapsed := time.Since(start)
	log.Default().Printf("Completed %d iterations of %s in %v", repeats*len(data), name, elapsed)
}

const (
	UserCount = 100000
	Repeats   = 100
)

func main() {
	data := generateData(UserCount)
	// Run benchmarks
	test(run_maphash, data, Repeats, "maphash")
	test(run_hmac_sha256, data, Repeats, "hmac-sha256")
}
