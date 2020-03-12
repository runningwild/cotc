package vote

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/runningwild/cotc/types"
)

type Vote struct {
	RoundSize        int
	MaxRounds        int
	Min, Max         int
	ImmediateResults bool
	Candidates       []Candidate
	Statements       []types.Statement

	validated bool
}

type Candidate struct {
	Name        string
	Description []string
}

type User struct {
	Name string
}

func (v *Vote) validate() {
	if v.validated {
		return
	}
	for i := range v.Statements {
		sort.Ints(v.Statements[i].Candidates)
	}
}

// Ballots[e] is the ballot for some member, e, of the electorate.  Choice[e][i] is some subset of the candidates
// that the voter strictly preferred to Ballots[e][i+k] for k >= 1.
func (v *Vote) NextQuestion(prefs []types.Preference, rng *rand.Rand) []int {
	v.validate()
	weight := v.CandidateGrid(prefs)
	for i := range weight {
		for j := range weight {
			if weight[i][j] > weight[j][i] {
				weight[j][i] = weight[i][j]
			}
		}
	}
	FloydWarshallSchulze(weight)
	minData := 1

	// knowledge will be a rough estimate of how many people we've already compared a candidate to
	var knowledge []int
	for i := range weight {
		count := 0
		for j := range weight {
			if i == j {
				continue
			}
			if weight[i][j] >= float64(minData) {
				count++
			}
		}
		knowledge = append(knowledge, count)
	}

	var needy [][2]int
	for i := range weight {
		for j := range weight {
			if weight[i][j] < float64(minData) {
				needy = append(needy, [2]int{i, j})
			}
		}
	}
	rng.Shuffle(len(needy), func(i, j int) { needy[i], needy[j] = needy[j], needy[i] })
	sort.SliceStable(needy, func(i, j int) bool {
		a := knowledge[needy[i][0]] - knowledge[needy[i][1]]
		b := knowledge[needy[j][0]] - knowledge[needy[j][1]]
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		return a > b
	})
	// rng.Shuffle(len(needy), func(i, j int) { needy[i], needy[j] = needy[j], needy[i] })

	candidateToStatements := make([][]int, len(v.Candidates))
	for i := range v.Statements {
		if len(v.Statements[i].Candidates) != 1 {
			continue
		}
		c := v.Statements[i].Candidates[0]
		candidateToStatements[c] = append(candidateToStatements[c], i)
	}
	var pairs [][2]int
	if len(needy) > 0 {
		roundSize := v.RoundSize
		if roundSize <= 2 {
			roundSize = 2
		}
		target := make(map[int]bool)
		for len(target) < roundSize && len(needy) > 0 {
			target[needy[0][0]] = true
			if len(target) < roundSize {
				target[needy[0][1]] = true
			}
			needy = needy[1:]
		}
		for len(target) < roundSize {
			target[rng.Intn(len(v.Candidates))] = true
		}
		var candidates []int
		for c := range target {
			candidates = append(candidates, c)
		}
		sort.Ints(candidates)
		var statements []int
		for _, c := range candidates {
			ss := candidateToStatements[c]
			statements = append(statements, ss[rng.Intn(len(ss))])
		}
		return statements
	} else {
		if len(prefs) >= v.MaxRounds {
			return nil
		}
		{
			scores := v.Score(prefs)
			if scores[v.Min-1].Quality > math.Pow(0.99, float64(len(v.Candidates))) {
				return nil
			}
			var roundSize = 2
			if v.RoundSize > roundSize {
				roundSize = v.RoundSize
			}
			usedStatements := make(map[int]int)
			usedCandidates := make(map[int]int)
			for _, pref := range prefs {
				ss := append([]int{pref.A}, pref.B...)
				for i := range ss {
					usedStatements[ss[i]]++
					usedCandidates[v.Statements[ss[i]].Candidates[0]]++
				}
			}

			round := choices(usedCandidates, scores, roundSize, rng)
			// Now assign statements to each candidate, but prefer an assignment that reduces the
			// number of pairs of statements the user sees repeated.
			var bestStatements []int
			var lowestUsed int
			for i := 0; i < 5; i++ {
				var statements []int
				for _, c := range round {
					ss := candidateToStatements[scores[c].Candidate]
					statements = append(statements, ss[rng.Intn(len(ss))])
				}
				var used int
				for j := range statements {
					used += usedStatements[statements[j]]
				}
				if used < lowestUsed || bestStatements == nil {
					lowestUsed = used
					bestStatements = statements
				}
			}
			return bestStatements
		}
		return nil
		// Phase 2
		// Find the set of leaders and pick statements that differentiate them.
		g := v.CandidateGrid(prefs)
		SchulzeStrictify(g)
		FloydWarshallSchulze(g)
		unbeatenList := v.UnbeatenLists(g, 1)
		cannotBeat := make([][]bool, len(g))
		for i := range cannotBeat {
			cannotBeat[i] = make([]bool, len(g))
		}
		for a := range unbeatenList {
			fmt.Printf("Unbeaten(%d): %v\n", a, unbeatenList[a])
			for _, b := range unbeatenList[a] {
				cannotBeat[a][b] = true
			}
		}
		leaders := v.SmithSet(g, 1)
		if len(leaders) <= v.Max {
			return nil
		}
		var useful []int
		for i := range v.Statements {
			if len(intersect(leaders, v.Statements[i].Candidates)) == 0 {
				continue
			}
			useful = append(useful, i)
		}
		for _, i := range useful {
			for _, j := range useful {
				if i == j {
					continue
				}
				if len(intersect(v.Statements[i].Candidates, v.Statements[j].Candidates)) > 0 {
					continue
				}
				pairs = append(pairs, [2]int{i, j})
			}
		}
		rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
		sort.SliceStable(pairs, func(i, j int) bool {
			iscore := 0
			for _, x := range v.Statements[pairs[i][0]].Candidates {
				for _, y := range v.Statements[pairs[i][1]].Candidates {
					if cannotBeat[x][y] && cannotBeat[y][x] {
						iscore++
					}
				}
			}
			jscore := 0
			for _, x := range v.Statements[pairs[j][0]].Candidates {
				for _, y := range v.Statements[pairs[j][1]].Candidates {
					if cannotBeat[x][y] && cannotBeat[y][x] {
						jscore++
					}
				}
			}
			return iscore > jscore
		})
	}

	used := make(map[string]bool)
	for _, pref := range prefs {
		used[pref.ID()] = true
	}
	for i, pair := range pairs {
		p := types.Preference{pair[0], []int{pair[1]}}
		if used[p.ID()] {
			continue
		}
		return pairs[i][:]
	}
	fmt.Printf("FAILED TO DISTINGUISH!!!\n")
	return nil
}

type Score struct {
	Candidate int
	Quality   float64
}

func choices(usedCandidates map[int]int, scores []Score, numChoices int, rng *rand.Rand) []int {
	var best float64
	var choice []int
	for i := 0; i < 10; i++ {
		round := singleChoice(scores, numChoices, rng)
		s := scoreChoice(usedCandidates, round, scores)
		if s > best {
			best = s
			choice = round
		}
	}
	return choice
}

func scoreChoice(usedCandidates map[int]int, choice []int, scores []Score) float64 {
	round := make([]Score, len(choice))
	for i, c := range choice {
		round[i] = scores[c]
	}
	var sum float64
	for _, c := range round {
		sum += c.Quality
	}
	var deltaProd float64 = 1
	for i := 0; i < len(round)-1; i++ {
		deltaProd *= math.Pow(2, round[i].Quality-round[i+1].Quality)
	}
	var repeatFactor float64 = 1
	for i := range choice {
		repeatFactor *= math.Pow(1.1, -float64(usedCandidates[choice[i]]))
	}
	return sum * deltaProd * repeatFactor
}

func singleChoice(scores []Score, numChoices int, rng *rand.Rand) []int {
	target := make(map[int]bool)
	baseline := 0.0
	for len(target) < numChoices {
		total := 0.0
		index := 0
		for i := range scores {
			if target[i] {
				continue
			}
			total += scores[i].Quality + baseline
			if r := rng.Float64(); r < (scores[i].Quality+baseline)/total {
				index = i
			}
		}
		target[index] = true
		baseline += 0.01
	}
	var candidates []int
	for c := range target {
		candidates = append(candidates, c)
	}
	sort.Ints(candidates)
	return candidates
}

func (v *Vote) Score(prefs []types.Preference) []Score {
	g := v.CandidateGrid(prefs)
	SchulzeStrictify(g)
	FloydWarshallSchulze(g)
	var scores []Score
	favors := make([]int, len(g))
	total := 0
	for i := range g {
		s := Score{Candidate: i}
		for j := range g[i] {
			if i == j {
				continue
			}
			if g[i][j] > g[j][i] {
				favors[i] += int(g[i][j] - g[j][i])
			}
			total += int(g[i][j] + g[j][i])
		}
		scores = append(scores, s)
	}
	for i := range scores {
		ratio := float64(favors[i]) / float64(total)
		scores[i].Quality = 1 - math.Pow(1-ratio, 1+float64(total)/float64(len(v.Candidates)))
	}
	sort.Slice(scores, func(i, j int) bool {
		{
			return scores[i].Quality > scores[j].Quality
		}
	})
	return scores
}

func (v *Vote) Top(prefs []types.Preference, margin float64) []int {
	g := v.CandidateGrid(prefs)
	SchulzeStrictify(g)
	FloydWarshallSchulze(g)
	smith := v.SmithSet(g, margin)
	if len(smith) < v.Min || len(smith) > v.Max {
		unbeaten := v.UnbeatenLists(g, margin)
		scores := make([][2]int, len(unbeaten))
		for i := range scores {
			scores[i] = [2]int{i, len(unbeaten[i])}
		}
		sort.Slice(scores, func(i, j int) bool {
			return scores[i][1] < scores[j][1]
		})
		max := -1
		pos := -1
		for i := v.Min - 1; i <= v.Max-1; i++ {
			if gap := scores[i+1][1] - scores[i][1]; gap > max {
				max = gap
				pos = i + 1
			}
		}
		smith = make([]int, pos)
		for i := range smith {
			smith[i] = scores[i][0]
		}
	}
	return smith
}

func intersect(a, b []int) []int {
	ai, bi := 0, 0
	var isect []int
	for ai < len(a) && bi < len(b) {
		if a[ai] == b[bi] {
			isect = append(isect, a[ai])
			ai++
			bi++
			continue
		}
		if a[ai] < b[bi] {
			ai++
		} else {
			bi++
		}
	}
	return isect
}

func (v *Vote) Rank(prefs []types.Preference, margin float64) []float64 {
	g := v.CandidateGrid(prefs)
	SchulzeStrictify(g)
	FloydWarshallSchulze(g)
	cur := make([]float64, len(v.Candidates))
	for i := range cur {
		cur[i] = 1.0 / float64(len(cur))
	}
	N := 100
	for i := 0; i < N; i++ {
		total := 0.0
		for i := range cur {
			cur[i] += 0.1
			total += cur[i]
		}
		for i := range cur {
			cur[i] /= total
		}
		next := make([]float64, len(cur))
		total = 0
		for i := range g {
			for j := range g[i] {
				total += g[i][j]
			}
		}
		if total > 0 {
			for j := range g {
				for i := range g {
					next[j] += (g[j][i] / total) * cur[i]
				}
			}
		}
		copy(cur, next)
	}

	total := 0.0
	for i := range cur {
		total += cur[i]
	}
	for i := range cur {
		cur[i] /= total
	}

	return cur
}

func (v *Vote) SmithSet(g [][]float64, margin float64) []int {
	unbeaten := v.UnbeatenLists(g, margin)
	var smithSet []int
	for i := range unbeaten {
		in := map[int]bool{i: true}
		out := make(map[int]bool)
		for _, c := range unbeaten[i] {
			out[c] = true
		}
		for len(out) > 0 {
			// Add everything unbeaten to the smith set.
			for c := range out {
				in[c] = true
			}
			// Recalculate the unbeaten set
			out = make(map[int]bool)
			for cin := range in {
				for _, u := range unbeaten[cin] {
					if !in[u] {
						out[u] = true
					}
				}
			}
		}
		if len(in) < len(smithSet) || smithSet == nil {
			smithSet = nil
			for c := range in {
				smithSet = append(smithSet, c)
			}
		}
	}
	sort.Ints(smithSet)
	return smithSet
}

func (v *Vote) UnbeatenLists(g [][]float64, margin float64) [][]int {
	var unbeaten [][]int
	for i := range g {
		var u []int
		for j := range g[i] {
			if i == j {
				continue
			}
			if g[i][j] <= g[j][i]+margin {
				u = append(u, j)
			}
		}
		unbeaten = append(unbeaten, u)
	}
	return unbeaten
}

func (v *Vote) margin(res [][]int) int {
	scores := make([]int, len(res))
	for i := range res {
		min := res[i][0]
		for j := range res {
			if i == j {
				continue
			}
			if res[i][j] < min {
				min = res[i][j]
			}
		}
		scores[i] = min
	}
	sort.Ints(scores)
	return scores[len(scores)-1] - scores[len(scores)-2]
}

func printGrid(title string, g [][]float64) {
	header := make([]int, len(g))
	for i := range header {
		header[i] = i
	}
	fmt.Printf("   %v - %q\n", header, title)
	for i := range g {
		fmt.Printf("%d: %2.1v\n", i, g[i])
	}
}

func zeroDiagonal(g [][]int) {
	for i := range g {
		g[i][i] = 0
	}
}

func SchulzeStrictify(g [][]float64) {
	p := makeSquare(len(g))
	for i := range p {
		copy(p[i], g[i])
	}
	for i := range p {
		for j := range p {
			if g[i][j] > g[j][i] {
				// Which DO I do!?
				//p[i][j] = g[i][j] - g[j][i]
				p[i][j] = g[i][j]
			} else {
				p[i][j] = 0
			}
		}
	}
	for i := range p {
		copy(g[i], p[i])
	}
}

func FloydWarshallSchulze(g [][]float64) {
	for k := range g {
		for i := range g {
			for j := range g {
				if i == k || j == k {
					continue
				}
				min := g[i][k]
				if g[k][j] < min {
					min = g[k][j]
				}
				if min > g[i][j] {
					g[i][j] = min
				}
			}
		}
	}
}

func init() {
	math.Log(2)
}

// Create matrix where d[i][j] is the number of voters that prefer candidate i to candidate j.
func (v *Vote) CandidateGrid(prefs []types.Preference) [][]float64 {
	d := makeSquare(len(v.Candidates))
	for _, pref := range prefs {
		a := v.Statements[pref.A].Candidates
		for _, c := range pref.B {
			b := v.Statements[c].Candidates
			for _, x := range a {
				for _, y := range b {
					// d[x][y]+=math.Log(float64(len(a)+len(b))) /  math.Log(2)
					d[x][y]++
				}
			}
		}
	}
	for i := range d {
		d[i][i] = 0
	}
	return d
}

func makeSquare(n int) [][]float64 {
	p := make([][]float64, n)
	for i := range p {
		p[i] = make([]float64, n)
	}
	return p
}
