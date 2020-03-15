package types

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"sort"

	"cloud.google.com/go/datastore"
	"golang.org/x/crypto/sha3"
)

type User struct {
	Email     string
	FirstName string
	FullName  string
}

func (u *User) Key() *datastore.Key {
	hash := make([]byte, 8)
	sha3.ShakeSum256(hash, []byte(fmt.Sprintf("%s:%s:%s", u.Email, u.FirstName, u.FullName)))
	return datastore.NameKey("user", base64.URLEncoding.EncodeToString(hash), nil)
}

func (u *User) Hash() string {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	enc.Encode(u)
	hash := make([]byte, 8)
	sha3.ShakeSum256(hash, buf.Bytes())
	return base64.URLEncoding.EncodeToString(hash)
}

type GICMPData struct {
	Statements []string
	Groups     []GICMPGroup
}

type GICMPGroup struct {
	Name      string
	Questions []int
	Blurbs    []string
}

type GICMPResponse struct {
	Ratings []int
	Results []int
}

type ExperiencesResponse struct {
	Results []ExperienceInfo
}

type ExperienceInfo struct {
	ExperienceID string
	Response     string
}

type Experiences struct {
	Statements []ExperienceEntry
}

type ExperienceEntry struct {
	ID   string
	Text string
}

type SurveyResponse struct {
	Preferences []Preference
	Results     []int
}

type SkillsResponse struct {
	Results []SkillInfo
}

type SkillInfo struct {
	Skill      string
	Experience string
}

// A Preference indicates that a user prefers Statement A to all Statements B.
type Preference struct {
	A int
	B []int
}

func (p *Preference) ID() string {
	v := append([]int{p.A}, p.B...)
	sort.Ints(v)
	return fmt.Sprintf("%v", v)
}

type Statement struct {
	Text       string
	Candidates []int
}
