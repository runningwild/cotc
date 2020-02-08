package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/runningwild/cotc/keeper"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/sha3"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

func main() {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, "churchofthecity")
	if err != nil {
		panic(fmt.Sprintf("WHAT: %v", err))
	}
	tk, err := keeper.New("static")
	if err != nil {
		panic(fmt.Sprintf("failed to create template keeper: %v", err))
	}

	r := mux.NewRouter()
	surveys := []string{"enneagram", "strengths", "gifts"}
	for _, name := range surveys {
		var v vote.Vote
		data, err := ioutil.ReadFile("static/" + name + ".json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
		//http.HandleFunc("/survey/" + name, voteHandler(&v))
		r.HandleFunc("/survey/"+name, voteHandler(&v, client, name, tk))
	}

	{
		data, err := ioutil.ReadFile("static/gicmp.json")
		if err != nil {
			panic(err)
		}
		var g types.GICMPData
		if err := json.Unmarshal(data, &g); err != nil {
			panic(err)
		}
		r.HandleFunc("/gicmp", gicmpHandler(&g, client, tk))
	}

	r.HandleFunc("/core", coreHandler(client, surveys, tk))
	r.HandleFunc("/skills", skillsHandler(client, tk))
	r.HandleFunc("/experiences", experiencesHandler(client, tk))
	r.HandleFunc("/_ah/health", healthCheckHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.Handle("/", r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "ok")
}

func getUser(client *datastore.Client, req *http.Request) (*types.User, error) {
	userhash := req.URL.Query().Get("user")
	if userhash == "" {
		return nil, fmt.Errorf("no user specified")
	}
	userkey := datastore.NameKey("user", userhash, nil)
	var user types.User
	if err := client.Get(req.Context(), userkey, &user); err != nil {
		return nil, fmt.Errorf("user unknown: %w", err)
	}
	return &user, nil
}

func encodePreference(p types.Preference) string {
	buf := bytes.NewBuffer(nil)
	varint := make([]byte, 8)
	n := binary.PutVarint(varint, int64(p.A))
	binary.Write(buf, binary.LittleEndian, varint[0:n])
	varint = make([]byte, 8)
	n = binary.PutVarint(varint, int64(len(p.B)))
	binary.Write(buf, binary.LittleEndian, varint[0:n])
	for _, v := range p.B {
		varint := make([]byte, 8)
		n := binary.PutVarint(varint, int64(v))
		binary.Write(buf, binary.LittleEndian, varint[0:n])
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes())
}

func decodePreference(str string) (*types.Preference, error) {
	data, err := base64.URLEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data)
	a, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	length, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, fmt.Errorf("%v", err)
	}
	var bs []int
	for i := 0; i < int(length); i++ {
		b, err := binary.ReadVarint(buf)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}
		bs = append(bs, int(b))
	}
	return &types.Preference{A: int(a), B: bs}, nil
}

func easyHashObj(obj interface{}) string {
	buf := bytes.NewBuffer(nil)
	gob.NewEncoder(buf).Encode(obj)
	hash := make([]byte, 8)
	sha3.ShakeSum256(hash, buf.Bytes())
	enc := base64.URLEncoding.EncodeToString(hash)
	return enc
}
