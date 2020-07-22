package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	json "github.com/json-iterator/go"
)

type SubReddit struct {
	Data struct {
		Children []struct {
			Data struct {
				Title string `json:"title"`
			}
		}
	}
}

func (sr *SubReddit) getData(url string) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.141 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(sr)
	if err != nil {
		log.Fatal(err)
	}
}

// [FS] [USA] Rick Owens Ramones
// Remove the []tag and limit to 5 words
func getTerm(title string) string {
	a := strings.Split(title, "]")
	b := a[len(a)-1]
	c := strings.ReplaceAll(b, "&amp;", "&")
	d := strings.Split(c, " ")
	if len(d) > 5 {
		d = d[0:6]
	}
	e := strings.Join(d, "+")
	return strings.TrimSpace(e)
}

var baseUrl string = "https://rafteal.netlify.app/.netlify/functions/timba?q="

func main() {
	const url string = "https://www.reddit.com/r/FashionRepsBST.json"

	var sr SubReddit
	sr.getData(url)
	// Skip first 2 post - blanket filter pinned posts
	var allTerms []string
	for _, post := range sr.Data.Children[2:] {
		allTerms = append(allTerms, getTerm(post.Data.Title))
	}

	ch := make(chan Product)

	for _, t := range allTerms {
		go request(ch, t)

	}

	products := []Product{}
	for range allTerms {
		p := <-ch
		if p.Img == "" {
			continue
		}
		products = append(products, p)
	}

	f, err := os.Create("site/index.html")
	if err != nil {
		log.Println(err)
		return
	}
	var tpl *template.Template = template.Must(template.ParseFiles("ref/tpl.gohtml"))
	tpl.ExecuteTemplate(f, "tpl.gohtml", products)
}

type Product struct {
	Title       string
	Url         string
	Img         string
	Description string
	Currency    string
	Price       string
}

func request(ch chan Product, u string) {

	p := Product{}
	url := baseUrl + u
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Failed request")
		ch <- p
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- p
		return
	}
	if !strings.Contains(string(body), "url") {
		ch <- p
		return
	}

	// Get first url by cutting evertything before `"url":"` and after `"},{"title"`
	s1 := strings.Split(string(body), `"url":"`)[1]
	p.Url = strings.Split(s1, `"`)[0]

	rProduct, err := http.Get(p.Url)
	if err != nil {
		log.Println("Failed request to product")
		ch <- p
		return
	}
	doc, err := goquery.NewDocumentFromResponse(rProduct)
	if err != nil {
		log.Println(err)
		ch <- p
		return
	}
	imgMeta := doc.Find(`meta[property*="image"][content*="/product"]`)
	img, _ := imgMeta.Attr("content")
	if img == "" {
		ch <- p
		return
	}
	if strings.HasPrefix(img, "http:") {
		img = strings.Replace(img, "http:", "https:", 1)
	}
	if strings.HasPrefix(img, "//") {
		img = "https:" + img
	}
	p.Img = img

	priceMeta := doc.Find(`meta[property*="amount"]`)
	price, _ := priceMeta.Attr("content")
	p.Price = price

	currMeta := doc.Find(`meta[property*="currency"]`)
	curr, _ := currMeta.Attr("content")
	p.Currency = curr

	descMeta := doc.Find(`meta[property*="description"]`)
	desc, _ := descMeta.Attr("content")
	p.Description = desc

	titleMeta := doc.Find(`meta[property*="title"]`)
	title, _ := titleMeta.Attr("content")
	//Cut too long title
	if len(title) > 75 {
		title = title[0:75] + "..."
	}
	p.Title = title
	ch <- p
}
