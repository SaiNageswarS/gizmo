package mupdf

type stextJSON struct {
	Pages []Page `json:"pages"`
}

type Page struct {
	Blocks []Block `json:"blocks"`
}

type Block struct {
	Type  string `json:"type"`
	BBox  BBox   `json:"bbox"`
	Lines []Line `json:"lines"`
}

type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Line struct {
	WMode int     `json:"wmode"`
	BBox  BBox    `json:"bbox"`
	Font  Font    `json:"font"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Text  string  `json:"text"`
}

type Font struct {
	Name   string  `json:"name"`
	Family string  `json:"family"`
	Weight string  `json:"weight"`
	Style  string  `json:"style"`
	Size   float64 `json:"size"`
}
