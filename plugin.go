package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/drone/drone-template-lib/template"
)

type (
	Repo struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}

	Build struct {
		Tag     string `json:"tag"`
		Event   string `json:"event"`
		Number  int    `json:"number"`
		Commit  string `json:"commit"`
		Ref     string `json:"ref"`
		Branch  string `json:"branch"`
		Author  string `json:"author"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Link    string `json:"link"`
		Started int64  `json:"started"`
		Created int64  `json:"created"`
	}

	Config struct {
		Method      string
		Username    string
		Password    string
		ContentType string
		Template    string
		Headers     []string
		URLs        []string
		ValidCodes  []int
		Debug       bool
		SkipVerify  bool
	}

	Job struct {
		Started int64
	}

	Plugin struct {
		Repo   Repo
		Build  Build
		Config Config
		Job    Job
	}
)

func (p Plugin) Exec() error {
	var (
		buf bytes.Buffer
		b   []byte
	)

	if p.Config.Template == "" {
		data := struct {
			Repo  Repo  `json:"repo"`
			Build Build `json:"build"`
		}{p.Repo, p.Build}

		if err := json.NewEncoder(&buf).Encode(&data); err != nil {
			fmt.Printf("Error: Failed to encode JSON payload. %s\n", err)
			return err
		}

		b = buf.Bytes()
	} else {
		txt, err := template.RenderTrim(p.Config.Template, p)

		if err != nil {
			return err
		}

		text := txt
		b = []byte(text)
	}

	// build and execute a request for each url.
	// all auth, headers, method, template (payload),
	// and content_type values will be applied to
	// every webhook request.

	for i, rawurl := range p.Config.URLs {
		uri, err := url.Parse(rawurl)

		if err != nil {
			fmt.Printf("Error: Failed to parse the hook URL. %s\n", err)
			os.Exit(1)
		}

		r := bytes.NewReader(b)
		req, err := http.NewRequest(p.Config.Method, uri.String(), r)

		if err != nil {
			fmt.Printf("Error: Failed to create the HTTP request. %s\n", err)
			return err
		}

		req.Header.Set("Content-Type", p.Config.ContentType)

		for _, value := range p.Config.Headers {
			header := strings.Split(value, "=")
			req.Header.Set(header[0], header[1])
		}

		if p.Config.Username != "" && p.Config.Password != "" {
			req.SetBasicAuth(p.Config.Username, p.Config.Password)
		}

		client := http.DefaultClient

		if p.Config.SkipVerify {
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
		}

		resp, err := client.Do(req)

		if err != nil {
			fmt.Printf("Error: Failed to execute the HTTP request. %s\n", err)
			return err
		}

		// Function checks if int is in slice of ints
		f := func(s []int, e int) bool {
			for _, a := range s {
				if a == e {
					return true
				}
			}
			return false
		}

		if len(p.Config.ValidCodes) > 0 && !f(p.Config.ValidCodes, resp.StatusCode) {
			err := fmt.Errorf("Response code %d not found among valid response codes", resp.StatusCode)
			fmt.Printf("Error: %s\n", err)
			return err
		}

		defer resp.Body.Close()

		if p.Config.Debug || resp.StatusCode >= http.StatusBadRequest {
			body, err := ioutil.ReadAll(resp.Body)

			if err != nil {
				fmt.Printf("Error: Failed to read the HTTP response body. %s\n", err)
			}

			if p.Config.Debug {
				fmt.Printf(
					debugFormat,
					i+1,
					req.URL,
					req.Method,
					req.Header,
					string(b),
					resp.Status,
					string(body),
				)
			} else {
				fmt.Printf(
					respFormat,
					i+1,
					req.URL,
					resp.Status,
					string(body),
				)
			}
		}
	}

	return nil
}
