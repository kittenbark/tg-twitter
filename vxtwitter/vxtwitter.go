package vxtwitter

import (
	"encoding/json"
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func New() *API {
	return &API{
		Timeout: time.Second * 5,
		Sync:    &sync.Mutex{},
		Client:  http.DefaultClient,
	}
}

type API struct {
	Timeout time.Duration
	Sync    *sync.Mutex
	Client  *http.Client
}

func (api *API) Get(url string) (*VxPost, error) {
	if api.Sync != nil {
		api.Sync.Lock()
		defer time.AfterFunc(api.Timeout, api.Sync.Unlock)
	}

	resp, err := api.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http.get '%s': %v", url, err)
	}
	defer resp.Body.Close()

	var post VxPost
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return nil, fmt.Errorf("json.decode '%s': %v", url, err)
	}

	return &post, nil
}

func (api *API) DownloadTempVx(url string) (files []string, dir string, post *VxPost, err error) {
	post, err = api.Get(url)
	if err != nil {
		return nil, "", nil, fmt.Errorf("api.download.get: %v", err)
	}

	dir, err = os.MkdirTemp("", "twitter_media_*")
	if err != nil {
		return nil, "", post, fmt.Errorf("api.download.mkdir: %v", err)
	}

	ret := []string{}
	client := grab.NewClient()
	client.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3.1 Safari/605.1.1"
	client.HTTPClient = api.Client
	for _, mediaURL := range post.MediaURLs {
		mediaURL += "?name=large"
		req, err := grab.NewRequest(dir, mediaURL)
		if err != nil {
			return ret, dir, post, fmt.Errorf("api.download.grab.newrequest for '%s': %v", mediaURL, err)
		}

		resp := client.Do(req)
		if err := resp.Err(); err != nil {
			return ret, dir, post, fmt.Errorf("api.download.grab.dorequest for '%s': %v", mediaURL, err)
		}
		ret = append(ret, resp.Filename)
	}

	return ret, dir, post, nil
}

func (api *API) DownloadTemp(url string) (files []string, dir string, post *VxPost, err error) {
	parsed, err := Vx(url)
	if err != nil {
		return nil, "", nil, fmt.Errorf("api.download.parseurl for '%s': %v", url, err)
	}
	return api.DownloadTempVx(parsed)
}

var VxRegex = regexp.MustCompile("https://api\\.vxtwitter\\.com/[a-zA-Z0-9_]{1,15}/status/[0-9]+")

func Vx(url string) (string, error) {
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	switch {
	case strings.HasPrefix(url, "https://twitter.com"):
		url = strings.Replace(url, "https://twitter.com", "https://api.vxtwitter.com", 1)
	case strings.HasPrefix(url, "https://x.com"):
		url = strings.Replace(url, "https://x.com", "https://api.vxtwitter.com", 1)
	case strings.HasPrefix(url, "https://vxtwitter.com"):
		url = strings.Replace(url, "https://vxtwitter.com", "https://api.vxtwitter.com", 1)
	default:
		return "", fmt.Errorf("url has to be 'twitter.com/*' (got '%s')", url)
	}

	parsed := VxRegex.FindAll([]byte(url), 1)
	if len(parsed) == 0 {
		return "", fmt.Errorf("url parsing failed")
	}
	return string(parsed[0]), nil
}

type VxMedia struct {
	AltText        string `json:"altText"`
	DurationMillis int64  `json:"duration_millis,omitempty"`
	Size           struct {
		Height int64 `json:"height"`
		Width  int64 `json:"width"`
	} `json:"size"`
	ThumbnailUrl string `json:"thumbnail_url"`
	Type         string `json:"type"`
	Url          string `json:"url"`
}

type VxPost struct {
	Date           string    `json:"date"`
	DateEpoch      int64     `json:"date_epoch"`
	Hashtags       []string  `json:"hashtags"`
	Likes          int64     `json:"likes"`
	MediaURLs      []string  `json:"mediaURLs"`
	MediaExtended  []VxMedia `json:"media_extended"`
	Replies        int64     `json:"replies"`
	Retweets       int64     `json:"retweets"`
	Text           string    `json:"text"`
	TweetID        string    `json:"tweetID"`
	TweetURL       string    `json:"tweetURL"`
	UserName       string    `json:"user_name"`
	UserScreenName string    `json:"user_screen_name"`
}
