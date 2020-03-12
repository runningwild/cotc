package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

func main() {
	doeverything("strengths")
	doeverything("enneagram")
	doeverything("gifts")
}
func doeverything(survey string) {
	fmt.Printf("%q\n", survey)
	var v vote.Vote
	data, err := ioutil.ReadFile(fmt.Sprintf("../static/%s.json", survey))
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &v); err != nil {
		panic(err)
	}
	N := 100
	start := 1
	totalQuestions := 0
	totalAccuracy := 0.0
	var mu sync.Mutex
	var wg sync.WaitGroup
	var hists []int
	for i := start; i < start+N; i++ {
		wg.Add(1)
		go func(roundNum int) {
			defer wg.Done()
			q, a, data, hist := round(&v, rand.New(rand.NewSource(int64(roundNum))))
			mu.Lock()
			totalQuestions += q
			totalAccuracy += a
			for len(hists) < len(hist) {
				hists = append(hists, 0)
			}
			for i := range hist {
				hists[i] += hist[i]
			}
			if len(data) > 0 {
				fmt.Printf("%s\n", data)
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Printf("avg %v\n", float64(totalQuestions)/float64(N))
	fmt.Printf("acc %v\n", totalAccuracy/float64(N))
	cutoff := int(math.Sqrt(float64(v.MaxRounds))) + 1
	fmt.Printf("Understat(%d): %v\n", cutoff, understat(hists, cutoff))
}

func understat(hist []int, cutoff int) float64 {
	total := 0
	for _, v := range hist {
		total += v
	}
	under := 0
	for _, v := range hist[0:cutoff] {
		under += v
	}
	return float64(under) / float64(total)
}

func score(cands, rank []int) float64 {
	val := 0.0
	for _, c := range cands {
		val += math.Sqrt(float64(rank[c]))
	}
	val /= float64(len(cands))
	val *= val
	return val
}

func round(v *vote.Vote, rng *rand.Rand) (questions int, accuracy float64, data string, hist []int) {
	rank := make([]int, len(v.Candidates))
	for i := range rank {
		rank[i] = i
	}
	rng.Shuffle(len(rank), func(i, j int) { rank[i], rank[j] = rank[j], rank[i] })
	file := make([]int, len(rank))
	for i := range rank {
		for j, v := range rank {
			if i == v {
				file[i] = j
			}
		}
	}
	var margin float64 = 1
	var prefs []types.Preference
	qCount := make(map[int]int)
	for {
		q := v.NextQuestion(prefs, rng)
		if q == nil {
			break
		}
		for _, i := range q {
			qCount[i]++
		}

		var opt int = -1
		var best float64
		for i, statement := range q {
			s := score(v.Statements[statement].Candidates, rank) + 2*rng.NormFloat64()
			if opt == -1 || s < best {
				best = s
				opt = i
			}
		}
		pref := types.Preference{A: q[opt]}
		for i := range q {
			if i != opt {
				pref.B = append(pref.B, q[i])
			}
		}
		scores := v.Score(prefs)
		if scores[v.Min-1].Quality > math.Pow(0.99, float64(len(v.Candidates))) {
			break
		}
		prefs = append(prefs, pref)
		g := v.CandidateGrid(prefs)
		vote.SchulzeStrictify(g)
		vote.FloydWarshallSchulze(g)
		unbeaten := v.UnbeatenLists(g, margin)
		sizes := make([]int, len(unbeaten))
		for i := range sizes {
			sizes[i] = len(unbeaten[i])
		}
		sort.Ints(sizes)
		if sizes[v.Min] <= v.Max {
			break
		}
	}
	hist = nil
	for _, count := range qCount {
		for len(hist) <= count {
			hist = append(hist, 0)
		}
		hist[count]++
	}
	scores := v.Score(prefs)
	var top []int
	for i := 0; i < v.Min; i++ {
		top = append(top, scores[i].Candidate)
	}
	// data += fmt.Sprintf("File: %v\n", file)
	// data += fmt.Sprintf("Final: %v\n", top)
	// if fmt.Sprintf("%v", top) == fmt.Sprintf("%v", file[0:len(top)]) {
	// 	fmt.Printf("ALL IN ONE\n")
	// }

	acc := 0.0
	opt := 0.0
	for i, c := range top {
		acc += math.Pow(float64(rank[c]), 2)
		opt += math.Pow(float64(i), 2)
	}
	acc /= float64(len(top))
	acc = math.Sqrt(acc)
	opt /= float64(len(top))
	opt = math.Sqrt(opt)
	acc = (acc+1)/(opt+1) - 1
	return len(prefs), acc, data, hist
}
