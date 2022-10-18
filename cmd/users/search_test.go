package users

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort"

	"github.com/flant/glaball/cmd/common"

	"github.com/stretchr/testify/assert"
	"github.com/xanzy/go-gitlab"
)

func TestSearch(t *testing.T) {
	mux, server, cli := common.Setup(t)
	defer common.Teardown(server)

	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodGet {
			t.Errorf("Request method: %s, want %s", got, http.MethodGet)
		}
		fmt.Fprint(w, TestData)
	})

	wg := limiter.NewLimiter(limiter.DefaultLimit)
	data := make(chan interface{})

	searchBy := "username"
	searchFieldValue := regexp.MustCompile("testuser2")

	fmt.Printf("Searching for user %q...\n", searchFieldValue)
	for _, h := range cli.Hosts {
		wg.Add(1)
		go listUsersSearch(h, searchBy, searchFieldValue, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data, cli.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{searchBy},
		StructType: gitlab.User{},
	})

	var user gitlab.User

	err := json.Unmarshal([]byte(`{
		"id": 122,
		"username": "testuser2",
		"name": "Test User 2",
		"state": "active",
		"avatar_url": "",
		"web_url": "https://gitlab.example.com/testuser2",
		"created_at": "2022-04-21T15:21:23.810+00:00",
		"bio": "",
		"location": "",
		"public_email": "",
		"skype": "",
		"linkedin": "",
		"twitter": "",
		"website_url": "",
		"organization": "",
		"job_title": "",
		"pronouns": "",
		"bot": false,
		"work_information": null,
		"followers": 0,
		"following": 0,
		"local_time": "5:45 PM",
		"last_sign_in_at": "2022-04-22T15:50:57.801+00:00",
		"confirmed_at": "2022-04-21T15:21:23.534+00:00",
		"last_activity_on": "2022-04-29",
		"email": "testuser2@example.com",
		"theme_id": 11,
		"color_scheme_id": 1,
		"projects_limit": 100000,
		"current_sign_in_at": "2022-04-27T07:54:55.035+00:00",
		"identities": [],
		"can_create_group": false,
		"can_create_project": true,
		"two_factor_enabled": true,
		"external": false,
		"private_profile": false,
		"commit_email": "testuser2@example.com",
		"is_admin": false,
		"note": ""
	}`), &user)

	assert.NoError(t, err)

	expected := []sort.Result{
		{
			Count:    1,
			Key:      searchFieldValue.String(),
			Elements: sort.Elements{sort.Element{Host: cli.Hosts[1], Struct: &user, Cached: false}},
			Cached:   false,
		},
		{
			Count:    1,
			Key:      searchFieldValue.String(),
			Elements: sort.Elements{sort.Element{Host: cli.Hosts[0], Struct: &user, Cached: false}},
			Cached:   false,
		}}

	assert.NotNil(t, results)
	assert.Equal(t, expected, results)

}

func TestSearchBlocked(t *testing.T) {
	mux, server, cli := common.Setup(t)
	defer common.Teardown(server)

	mux.HandleFunc("/api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodGet {
			t.Errorf("Request method: %s, want %s", got, http.MethodGet)
		}
		fmt.Fprint(w, TestData)
	})

	wg := limiter.NewLimiter(limiter.DefaultLimit)
	data := make(chan interface{})

	searchBy := "username"
	searchFieldValue := regexp.MustCompile("testuser3")

	fmt.Printf("Searching for user %q...\n", searchFieldValue)
	for _, h := range cli.Hosts {
		wg.Add(1)
		go listUsersSearch(h, searchBy, searchFieldValue, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
			Blocked: gitlab.Bool(true),
		}, wg, data, cli.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{searchBy},
		StructType: gitlab.User{},
	})

	var user gitlab.User

	err := json.Unmarshal([]byte(`{
		"id": 121,
		"username": "testuser3",
		"name": "Test User 3",
		"state": "blocked",
		"avatar_url": "",
		"web_url": "https://gitlab.example.com/testuser3",
		"created_at": "2022-04-21T15:21:23.810+00:00",
		"bio": "",
		"location": "",
		"public_email": "",
		"skype": "",
		"linkedin": "",
		"twitter": "",
		"website_url": "",
		"organization": "",
		"job_title": "",
		"pronouns": "",
		"bot": false,
		"work_information": null,
		"followers": 0,
		"following": 0,
		"local_time": "5:45 PM",
		"last_sign_in_at": "2022-04-22T15:50:57.801+00:00",
		"confirmed_at": "2022-04-21T15:21:23.534+00:00",
		"last_activity_on": "2022-04-29",
		"email": "testuser3@example.com",
		"theme_id": 11,
		"color_scheme_id": 1,
		"projects_limit": 100000,
		"current_sign_in_at": "2022-04-27T07:54:55.035+00:00",
		"identities": [],
		"can_create_group": false,
		"can_create_project": true,
		"two_factor_enabled": true,
		"external": false,
		"private_profile": false,
		"commit_email": "testuser3@example.com",
		"is_admin": false,
		"note": ""
	}`), &user)

	assert.NoError(t, err)

	expected := []sort.Result{
		{
			Count:    1,
			Key:      searchFieldValue.String(),
			Elements: sort.Elements{sort.Element{Host: cli.Hosts[0], Struct: &user, Cached: false}},
			Cached:   false,
		},
		{
			Count:    1,
			Key:      searchFieldValue.String(),
			Elements: sort.Elements{sort.Element{Host: cli.Hosts[1], Struct: &user, Cached: false}},
			Cached:   false,
		}}

	assert.NotNil(t, results)
	assert.Equal(t, expected, results)

}
