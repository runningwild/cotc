package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/jung-kurt/gofpdf"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

const targetUser = "4bdvv3KySoM="

func main() {
	if err := doit(context.Background()); err != nil {
		panic(err)
	}
}

func doit(ctx context.Context) error {
	client, err := datastore.NewClient(ctx, "churchofthecity")
	if err != nil {
		panic(fmt.Sprintf("WHAT: %v", err))
	}

	var user types.User
	if err := client.Get(ctx, datastore.NameKey("user", targetUser, nil), &user); err != nil {
		return fmt.Errorf("couldn't find user: %w", err)
	}

	var survey surveyData
	data, err := ioutil.ReadFile("../static/gicmp.json")
	if err != nil {
		return fmt.Errorf("failed to load gicmp data: %w")
	}
	if err := json.Unmarshal(data, &survey.gicmp); err != nil {
		return fmt.Errorf("failed to decode gicmp data: %w", err)
	}

	survey.surveys = make(map[string]*vote.Vote)
	for _, name := range []string{"gifts", "enneagram", "strengths"} {
		data, err := ioutil.ReadFile("../static/" + name + ".json")
		if err != nil {
			return fmt.Errorf("failed to load %s data: %w", name, err)
		}
		var v vote.Vote
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("failed to decode %s data: %w", name, err)
		}
		survey.surveys[name] = &v
	}

	{
		data, err := ioutil.ReadFile("../static/experiences.json")
		if err != nil {
			return fmt.Errorf("failed to load experiences data: %w", err)
		}
		if err := json.Unmarshal(data, &survey.experiences); err != nil {
			return fmt.Errorf("failed to decode experiences data: %w", err)
		}
	}

	userData, err := getAllUserData(ctx, client, &user)
	if err != nil {
		return err
	}

	pdf := singleUserReport(userData, survey)
	pdf.OutputFileAndClose("hello.pdf")
	return nil
}

func singleUserReport(user *allUserData, surveyData surveyData) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	left, _, right, _ := pdf.GetMargins()

	pdf.SetFontLocation(".")
	pdf.AddFont("Luminari", "", "Luminari.json")

	var h float64
	setFont := func(name, style string, size float64) {
		pdf.SetFont(name, style, size)
		_, h = pdf.GetFontSize()
	}
	setHeading := func() { setFont("Times", "", 36) }
	setSubheadingBold := func() { setFont("Times", "B", 20) }
	setSubheading := func() { setFont("Times", "", 20) }
	setStandard := func() { setFont("Times", "", 13.25) }
	setStandardBold := func() { setFont("Times", "B", 13.25) }

	// GIFTS on the upper left
	pdf.SetFont("Arial", "", 48)
	_, h = pdf.GetFontSize()
	pdf.SetHomeXY()
	pdf.Write(h, "GIFTS")

	// Name and email on the upper right
	pdf.SetFont("Luminari", "", 20)
	_, h = pdf.GetFontSize()
	pdf.WriteAligned(right-left, h, user.FirstName+" "+user.LastName, "R")
	pdf.Ln(h)
	pdf.SetFont("Arial", "", 20)
	pdf.WriteAligned(right-left, h, user.Email, "R")

	// Kingdom Values
	pdf.SetXY(left, 4*h)
	setHeading()
	pdf.Write(h, "Kingdom Values")
	pdf.Ln(2 * h)
	for i, group := range surveyData.gicmp.Groups {
		mode := []string{"Know", "Experience", "Confidence", "Apply"}
		res := user.gicmp.Results[i]
		if res < 0 || res > len(mode) {
			res = 0
		}
		setSubheadingBold()
		pdf.Write(h, group.Name+" - ")
		setSubheading()
		pdf.Write(h, mode[res])
		pdf.Ln(1.25 * h)
		setStandard()
		pdf.Write(h, group.Blurbs[res])
		pdf.Ln(1.75 * h)
	}

	pdf.AddPage()
	setHeading()
	pdf.Ln(1.25 * h)
	pdf.Write(h, "Spiritual Gifts")
	pdf.Ln(1.25 * h)
	for _, result := range user.surveys["gifts"].Results {
		setSubheadingBold()
		pdf.Write(h, surveyData.surveys["gifts"].Candidates[result].Name)
		pdf.Ln(h)
		setStandard()
		pdf.Write(h, surveyData.surveys["gifts"].Candidates[result].Description)
		pdf.Ln(1.5 * h)
	}
	pdf.Ln(2 * h)
	setSubheadingBold()
	pdf.Ln(1.5 * h)
	pdf.Write(h, "Enthusiasms")
	pdf.Ln(1.5 * h)
	idToStatement := make(map[string]string)
	for _, res := range surveyData.experiences.Statements {
		idToStatement[res.ID] = res.Text
	}
	idToResponse := make(map[string]string)
	for _, res := range user.experiences.Results {
		idToResponse[res.ExperienceID] = res.Response
	}
	writeFreeResponses := func(ids []string) {
		for _, id := range ids {
			text, ok := idToStatement[id]
			if !ok {
				fmt.Printf("didn't find experience with id %q\n", id)
				continue
			}
			response, ok := idToResponse[id]
			if !ok {
				fmt.Printf("didn't find response with id %q\n", id)
				continue
			}
			setStandardBold()
			pdf.Write(h, text)
			pdf.Ln(h)
			setStandard()
			pdf.Write(h, response)
			pdf.Ln(2 * h)
		}
	}
	writeFreeResponses([]string{"called to purposes", "called to places", "two needs", "cannot fail"})

	pdf.AddPage()
	setHeading()
	pdf.Write(h, "Strengths")
	pdf.Ln(1.25 * h)
	for _, result := range user.surveys["strengths"].Results {
		setSubheadingBold()
		pdf.Write(h, surveyData.surveys["strengths"].Candidates[result].Name)
		pdf.Ln(h)
		setStandard()
		pdf.Write(h, surveyData.surveys["strengths"].Candidates[result].Description)
		pdf.Ln(1.5 * h)
	}

	setHeading()
	pdf.Ln(h)
	pdf.Write(h, "The Enneagram")
	pdf.Ln(1.25 * h)
	for _, result := range user.surveys["enneagram"].Results {
		setSubheadingBold()
		pdf.Write(h, surveyData.surveys["enneagram"].Candidates[result].Name)
		pdf.Ln(h)
		setStandard()
		pdf.Write(h, surveyData.surveys["enneagram"].Candidates[result].Description)
		pdf.Ln(1.5 * h)
	}

	pdf.AddPage()
	setHeading()
	pdf.Write(h, "Skills and Experiences")
	pdf.Ln(1.5 * h)
	var work, hobby []string
	for _, res := range user.skills.Results {
		switch res.Experience {
		case "Both":
			hobby = append(hobby, res.Skill)
			work = append(work, res.Skill)
		case "Hobby":
			hobby = append(hobby, res.Skill)
		case "Work":
			work = append(work, res.Skill)
		}
	}
	sort.Strings(hobby)
	sort.Strings(work)
	setSubheadingBold()
	pdf.Write(h, "Work Skills")
	pdf.Ln(1.25 * h)
	setStandard()
	pdf.Write(h, strings.Join(work, ", "))
	setSubheadingBold()
	pdf.Ln(1.25 * h)
	pdf.Write(h, "Hobbies")
	pdf.Ln(1.25 * h)
	setStandard()
	pdf.Write(h, strings.Join(hobby, ", "))

	setSubheadingBold()
	pdf.Ln(1.25 * h)
	pdf.Write(h, "Experiences")
	pdf.Ln(1.25 * h)
	writeFreeResponses([]string{"places", "cultures", "family growing up", "childhood activities", "family adult", "adult hobbies", "jobs", "people", "experiences", "became a believer"})

	return pdf
}

func getAllUserData(ctx context.Context, client *datastore.Client, user *types.User) (*allUserData, error) {
	userData := allUserData{
		User: user,
	}

	if err := client.Get(ctx, datastore.NameKey("GICMPResponse", "highlander", user.Key()), &userData.gicmp); err != nil {
		return nil, fmt.Errorf("failed to get gicmp data: %w", err)
	}

	if err := client.Get(ctx, datastore.NameKey("SkillsResponse", "highlander", user.Key()), &userData.skills); err != nil {
		return nil, fmt.Errorf("failed to get skills data: %w", err)
	}

	if err := client.Get(ctx, datastore.NameKey("ExperiencesResponse", "highlander", user.Key()), &userData.experiences); err != nil {
		return nil, fmt.Errorf("failed to get experiences data: %w", err)
	}

	userData.surveys = make(map[string]types.SurveyResponse)
	for _, name := range []string{"gifts", "enneagram", "strengths"} {
		var sr types.SurveyResponse
		if err := client.Get(ctx, datastore.NameKey("SurveyResponse", name, user.Key()), &sr); err != nil {
			return nil, fmt.Errorf("failed to get %s survey response: %w", name, err)
		}
		userData.surveys[name] = sr
	}

	return &userData, nil
}

type allUserData struct {
	*types.User
	gicmp       types.GICMPResponse
	surveys     map[string]types.SurveyResponse
	experiences types.ExperiencesResponse
	skills      types.SkillsResponse
}

type surveyData struct {
	gicmp       types.GICMPData
	surveys     map[string]*vote.Vote
	experiences types.Experiences
}
