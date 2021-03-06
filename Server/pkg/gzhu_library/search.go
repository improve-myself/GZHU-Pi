/**
广州大学图书馆网站搜索查询接口
*/

package gzhu_library

import (
	"GZHU-Pi/env"
	"fmt"
	"github.com/astaxie/beego/logs"
	"github.com/go-redis/redis"
	jsoniter "github.com/json-iterator/go"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Book struct {
	ISBN      string `json:"ISBN" remark:"ISBN"`
	Author    string `json:"author" remark:"作者"`
	BookName  string `json:"book_name" remark:"书名"`
	CallNo    string `json:"call_No" remark:"索书号"`
	Copies    string `json:"copies" remark:"复本数量"`
	ID        string `json:"id" remark:"书籍id"`
	Image     string `json:"image" remark:"封面图片"`
	Loanable  string `json:"loanable" remark:"可借数量"`
	Publisher string `json:"publisher" remark:"出版社"`
	Source    string `json:"source" remark:"搜索源"`
}

/**
query:用户搜索的图书
searchPage:请求页数
[]*Book:返回Book类型的切片
*/
func BookSearch(query string, searchPage string) (result map[string]interface{}, err error) {

	if query == "" {
		return
	}
	//为0时会返回所有页数据
	if searchPage == "0" || searchPage == "" {
		searchPage = "1"
	}
	t1 := time.Now()
	u := "http://lib.gzhu.edu.cn:8080/bookle/"
	resp, err := http.PostForm(u, url.Values{"query": {query}, "searchPage": {searchPage}})
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logs.Error(err)
		return nil, err
	}
	logs.Debug("请求耗时：", time.Since(t1), u)

	//匹配book_info
	re := regexp.MustCompile(`<div class=book_info>([\s\S]*?)</div>`)
	bookInfo := re.FindAllStringSubmatch(string(bytes), -1)
	if bookInfo == nil {
		logs.Info(`正则匹配失败，返回为空，书名:%s`, query)
		return
	}

	var wg = sync.WaitGroup{}
	var books []*Book
	for i := 0; i < len(bookInfo); i++ {
		if len(bookInfo[i]) < 2 {
			logs.Error("非预期错误", bookInfo[i])
			continue
		}
		book := &Book{}

		//匹配book_info下的h2
		re = regexp.MustCompile(`<h2[\s\S]*?>([\s\S]*?)</h2>`)

		bookInfoH2 := re.FindAllStringSubmatch(bookInfo[i][1], -1)
		if bookInfoH2 == nil {
			logs.Warn(`正则匹配失败，返回为空`)
		} else { //书名
			re = regexp.MustCompile(`<a[\s\S]*?>[\s]*?([\S][\s\S]*?)</a>`)
			bookInfoH2A := re.FindAllStringSubmatch(bookInfoH2[0][1], -1)
			if bookInfoH2A == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.BookName = bookInfoH2A[0][1]
				book.BookName = strings.TrimSuffix(book.BookName, " /")

				//书籍id
				re = regexp.MustCompile(`[\d]{3,}`)
				bookID := re.FindAllStringSubmatch(bookInfoH2A[0][0], -1)
				if bookID == nil {
					logs.Warn(`正则匹配失败，返回为空`)
				} else {
					book.ID = bookID[0][0]
				}

				//搜索源
				re = regexp.MustCompile(`source=(.+)"`)
				bookResource := re.FindAllStringSubmatch(bookInfoH2A[0][0], -1)
				if bookResource == nil {
					logs.Warn(`正则匹配失败，返回为空`)
				} else {
					book.Source = bookResource[0][1]
				}
			}

			//作者
			re = regexp.MustCompile(`<span[\s\S]*?>(.*?)</span>`)
			bookAuthor := re.FindAllStringSubmatch(bookInfoH2[0][1], -1)
			if bookAuthor == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.Author = bookAuthor[0][1]
			}
		}

		//匹配book_info下的h4
		re = regexp.MustCompile(`<h4[\s\S]*?>[\s\S]*?</h4>`)
		bookInfoH4 := re.FindAllStringSubmatch(bookInfo[i][1], -1)
		if bookInfoH4 == nil {
			logs.Warn(`正则匹配失败，返回为空`)
		} else {
			//出版社
			re = regexp.MustCompile(`出版发行：([\s\S]*?)\r\n`)
			bookPublisher := re.FindAllStringSubmatch(bookInfoH4[0][0], -1)
			if bookPublisher == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.Publisher = bookPublisher[0][1]
				book.Publisher = strings.Replace(book.Publisher, ",", "", -1)
				book.Publisher = strings.Replace(book.Publisher, "O&#39", "", -1)
				book.Publisher = strings.Replace(book.Publisher, ";", "", -1)
				book.Publisher = strings.Replace(book.Publisher, " ", "", -1)
			}

			re = regexp.MustCompile(`&nbsp;&nbsp;[\s\S]*?&nbsp;&nbsp;`)
			bookISbnRow := re.FindAllStringSubmatch(bookInfoH4[0][0], -1)
			if bookISbnRow == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				//ISbn
				re = regexp.MustCompile(`[\d-]{10,}`)
				bookISbn := re.FindAllStringSubmatch(bookISbnRow[0][0], -1)
				if bookISbn == nil {
					logs.Warn(`正则匹配失败，返回为空`)
				} else {
					book.ISBN = bookISbn[0][0]
				}
				book.ISBN = strings.Replace(book.ISBN, "-", "", -1)
			}

			//复本数量
			re = regexp.MustCompile(`复本数.*([\d]+).*\n`)
			bookCopyNumber := re.FindAllStringSubmatch(bookInfoH4[1][0], -1)
			if bookCopyNumber == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.Copies = bookCopyNumber[0][1]
			}

			//可借数量
			re = regexp.MustCompile(`在馆数.*([\d]+).*\n`)
			bookCouldBeBorrow := re.FindAllStringSubmatch(bookInfoH4[1][0], -1)
			if bookCouldBeBorrow == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.Loanable = bookCouldBeBorrow[0][1]
			}

			//索书号
			re = regexp.MustCompile(`([A-Z][\s\S]*?)\r\n`)
			bookCallNumber := re.FindAllStringSubmatch(bookInfoH4[1][0], -1)
			if bookCallNumber == nil {
				logs.Warn(`正则匹配失败，返回为空`)
			} else {
				book.CallNo = bookCallNumber[0][1]
			}
		}
		//异步请求豆瓣接口获取图书封面
		time.Sleep(20 * time.Millisecond)
		wg.Add(1)
		go func() {
			key := fmt.Sprintf("gzhupi:book:cover:%s", book.ISBN)
			val, err := env.RedisCli.Get(key).Result()
			if err != nil && err != redis.Nil {
				logs.Error(err)
				return
			}
			if err == redis.Nil {
				book.Image = GetCover(book.ISBN)
				//加入缓存
				if book.Image != "" {
					logs.Debug("Set cache %s", key)
					err = env.RedisCli.Set(key, book.Image, 0).Err()
					if err != nil {
						logs.Error(err)
						return
					}
				}
			} else {
				//解析缓存
				logs.Debug("Hit cache %s", key)
				book.Image = val
			}
			wg.Done()
		}()

		books = append(books, book)
	}
	wg.Wait()

	var total = 0
	r1 := regexp.MustCompile(`(\d+)条结果`)
	totalMatch := r1.FindStringSubmatch(string(bytes))
	if len(totalMatch) == 2 {
		total, _ = strconv.Atoi(totalMatch[1])
	}

	result = map[string]interface{}{
		"books": books,
		"pages": math.Ceil(float64(total) / float64(10)),
		"total": total,
	}

	return result, nil
}

//提取豆瓣图书封面
func GetCover(ISBN string) (image string) {
	if ISBN == "" {
		return
	}
	t1 := time.Now()
	var URL = "https://douban.uieee.com/v2/book/isbn/" + ISBN
	client := http.Client{}
	request, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		logs.Error(err)
		return
	}
	const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.100 Safari/537.36"
	request.Header.Set("User-Agent", UA)
	resp, err := client.Do(request)
	if err != nil {
		logs.Error(err)
		return
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logs.Debug("请求耗时：", time.Since(t1), URL)

	image = jsoniter.Get(body, "image").ToString()
	return image
}
