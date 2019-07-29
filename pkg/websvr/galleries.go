package websvr

import (
	"fmt"
	"github.com/firefoxchan/local-ehentai/pkg/ehloader"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func galleries (replaceThumbs bool, thumbs string) func (writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		tmpl := template.New("galleries_extended.tmpl")
		t, e := tmpl.ParseFiles("template/galleries_extended.tmpl")
		if e != nil {
			_, _ = writer.Write([]byte(e.Error()))
			logger.Printf("Galleries template load error: %s", e)
			return
		}
		values, e := url.ParseQuery(request.URL.RawQuery)
		if e != nil {
			_, _ = writer.Write([]byte(e.Error()))
			logger.Printf("Galleries parse query error: %s", e)
			return
		}
		pq := parseQuery(values)
		logger.Printf("Galleries query: %v -> %+v", values, pq)
		gs, total := ehloader.SearchQ(pq.Offset, pq.Limit, pq.Q)
		maxPage := total / pq.Limit
		if total % pq.Limit != 0 {
			maxPage++
		}
		fromPage := pq.Page+1 -3
		toPage := fromPage +7
		pages := make([]int, 0)
		genPageFirst := true
		genPageLeftJumper := true
		genPageLast := true
		genPageRightJumper := true
		for p:=fromPage; p<=toPage; p++ {
			if p == 1 {
				genPageFirst = false
				genPageLeftJumper = false
			}
			if p == 2 {
				genPageLeftJumper = false
			}
			if p == maxPage {
				genPageLast = false
				genPageRightJumper = false
			}
			if p == maxPage-1 {
				genPageRightJumper = false
			}
			if p >=1 && p <= maxPage {
				pages = append(pages, p)
			}
		}
		data := map[string]interface{}{
			"Galleries": gs,
			"Total": total,
			"Page": pq.Page+1,
			"Pages": pages,
			"GenPageFirst": genPageFirst,
			"GenPageLast": genPageLast,
			"GenPageLeftJumper": genPageLeftJumper,
			"GenPageRightJumper": genPageRightJumper,
			"MaxPage": maxPage,
			"FSearch": pq.FSearch,
			"FCats": pq.FCats,
			"FCatsM": pq.FCatsM,
			"CategoryToCt": categoriesCtMap,
			"GenRatingStyle": genRatingStyle,
			"AddI": func(a int, b int) int { return a + b },
			"GenThumb": func(u string) string {
				if replaceThumbs {
					return genThumbs(u, thumbs)
				} else {
					return u
				}
			},
		}
		if e := t.Execute(writer, data); e != nil {
			_, _ = writer.Write([]byte(e.Error()))
			logger.Printf("Galleries render error: %s", e)
			return
		}
	}
}

func genThumbs (u string, dir string) string {
	parsedUrl, e := url.Parse(u)
	if e != nil {
		logger.Printf("Unable to parse url: %s\n", u)
		return u
	}
	paths := strings.Split(strings.TrimRight(parsedUrl.Path, "/"), "/")
	filename := paths[len(paths)-1]
	if _, e := os.Stat(filepath.Join(dir, filename)); e == nil {
		logger.Printf("Use thumbs cache: %s\n", filename)
		return thumbsPrefix + filename
	}
	filenames := strings.Split(filename, "_")
	if len(filenames) < 2 {
		logger.Printf("Unable to find thumbs cache: %s\n", u)
		return u
	}
	for _, tail := range []string {
		"250.jpg",
	} {
		filenames[len(filenames)-1] = tail
		filename := strings.Join(filenames, "_")
		if _, e := os.Stat(filepath.Join(dir, filename)); e == nil {
			logger.Printf("Use thumbs cache: %s\n", filename)
			return thumbsPrefix + filename
		}
	}
	logger.Printf("Unable to find thumbs cache: %s\n", u)
	return u
}

func genRatingStyle (rating32 float32) string {
	times := 0
	rating32 -= 5
	for rating32 < 0 {
		times++
		rating32 += 0.5
	}
	if math.Round(float64(rating32)*2) > 0 {
		times--
	}
	posX := (times / 2) * -16
	posY := ((times % 2) * -20) -1
	return fmt.Sprintf("background-position:%dpx %dpx", posX, posY)
}

var categoriesCtMap = map[string]string{
	"Misc": "ct1",
	"Doujinshi": "ct2",
	"Manga": "ct3",
	"Artist CG": "ct4",
	"Game CG": "ct5",
	"Image Set": "ct6",
	"Cosplay": "ct7",
	"Asian Porn": "ct8",
	"Non-H": "ct9",
	"Western": "cta",
}

var queryCategories = map[int64]string{
	1:   "misc",
	2:   "doujinshi",
	4:   "manga",
	8:   "artist cg",
	16:  "game cg",
	32:  "image set",
	64:  "cosplay",
	128: "asian porn",
	512: "western",
	256: "non-h",
}

type parsedQuery struct {
	Page int
	Offset int
	Limit int
	Q ehloader.Q
	FSearch string
	FCats int64
	FCatsM map[int64]bool
}

func parseQuery (values url.Values) parsedQuery {
	const limit = 10
	page, _ := strconv.ParseInt(values.Get("page"), 10, 64)
	offset := page * limit
	qs := make([]ehloader.Q, 0)
	fCats, _ := strconv.ParseInt(values.Get("f_cats"), 10, 64)
	fCatsM := map[int64]bool{}
	{
		categories := make([]string, 0)
		for mask, category := range queryCategories {
			if fCats & mask == 0 {
				fCatsM[mask] = true
				categories = append(categories, category)
			} else {
				fCatsM[mask] = false
			}
		}
		if len(categories) == 0 {
			// all disabled
			// -> all enabled
			fCats = 0
			fCatsM = map[int64]bool{}
		} else {
			categoryQs := make([]ehloader.Q, len(categories))
			for i, category := range categories {
				categoryQs[i] = ehloader.Eq(ehloader.TagKCategory, category)
			}
			qs = append(qs, ehloader.Or(categoryQs...))
		}
	}
	fSearch := strings.TrimSpace(values.Get("f_search"))
	{
		kvs := strings.Split(fSearch, ",")
		for _, kv := range kvs {
			key, value := ehloader.BuildKV(kv, ehloader.TagKAll)
			if value == "" {
				continue
			}
			if strings.HasSuffix(value, "$") {
				qs = append(qs, ehloader.Eq(key, value[:len(value)-1]))
			} else {
				qs = append(qs, ehloader.Like(key, value))
			}
		}
	}
	return parsedQuery{
		Page:   int(page),
		Offset: int(offset),
		Limit:  limit,
		Q:      ehloader.And(qs...),
		FSearch: fSearch,
		FCats: fCats,
		FCatsM: fCatsM,
	}
}
