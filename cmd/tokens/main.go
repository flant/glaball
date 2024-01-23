package tokens

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort/v3"
	"github.com/flant/glaball/pkg/util"
	"github.com/jmoiron/sqlx/reflectx"

	"github.com/flant/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	mapper              = reflectx.NewMapper("json")
	listProjectsOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	groupBy, sortBy     string
	orderBy             []string

	columns = []string{"Host.Project", "name", "scopes", "active", "expires_at", "Cached"}

	personalAccessTokensFormat = util.Dict{
		{
			Key:   "HOST",
			Value: "[%s]",
		},
		{
			Key:   "NAME",
			Value: "%s",
		},
		{
			Key:   "SCOPES",
			Value: "%v",
		},
		{
			Key:   "ACTIVE",
			Value: "[%t]",
		},
		{
			Key:   "EXPIRES",
			Value: "%s",
		},
		{
			Key:   "CACHED",
			Value: "[%s]",
		},
	}

	totalFormat = util.Dict{
		{
			Value: "Unique: %d",
		},
		{
			Value: "Total: %d",
		},
		{
			Value: "Errors: %d",
		},
	}
)

func NewCmd() *cobra.Command {
	typ := new(sort.Element[*gitlab.PersonalAccessToken])
	rt := reflect.TypeOf(typ.Struct)
	m := mapper.TypeMap(rt)
	validFields := make([]string, 0, len(m.Paths))
	for k, v := range m.Names {
		validFields = append(validFields, v.Name)
		fmt.Printf("%s\n", k)
	}
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Retrieve tokens",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Tokens()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&groupBy, "name", "path"), "group_by",
		"Return projects grouped by id, name, path, fields.")

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return projects sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&orderBy, "order_by", []string{"name"},
		fmt.Sprintf(`Return projects ordered by %s.`, strings.Join(validFields, ", ")))

	return cmd
}

func Tokens() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	typ := new(sort.Element[*gitlab.PersonalAccessToken])
	rt := reflect.TypeOf(typ)
	m := mapper.TypeMap(rt)

	rtstruct := reflect.TypeOf(typ.Struct)
	mstruct := mapper.TypeMap(rtstruct)

	fmt.Printf("\nvalid fields:\n")
	for k := range m.Paths {
		fmt.Printf("%s\n", k)
	}

	fmt.Printf("\nvalid fields:\n")
	for _, v := range mstruct.Paths {
		fmt.Printf("%s\n", v.Name)
	}

	// check columns
	fmt.Printf("\ncheck columns:\n")
	for _, p := range columns {
		if fi := m.GetByPath(p); fi == nil {
			if fi := m.GetByPath("Struct." + p); fi == nil {
				return fmt.Errorf("unknown field: %s", p)
			} else {
				fmt.Printf("%s\n", fi.Path)
			}
		} else {
			fmt.Printf("%s\n", fi.Path)
		}
	}

	// return nil

	wg := limiter.NewLimiter(limiter.DefaultLimit)
	data := make(chan sort.Element[*gitlab.PersonalAccessToken])
	for _, h := range common.Client.Hosts {
		fmt.Printf("Fetching user tokens from %s ...\n", h.URL)
		wg.Add(1)
		go listPersonalAccessTokens(h, gitlab.ListPersonalAccessTokensOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data, gitlab.WithContext(ctx), common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	if _, err := fmt.Fprintln(w, strings.Join(personalAccessTokensFormat.Keys(), "\t")); err != nil {
		return err
	}

	total := 0
	unique := 0

	results, err := sort.FromChannel(data, &sort.Options{
		SortBy:  "desc",
		GroupBy: "",
		OrderBy: []string{"name", "id"},
	})
	if err != nil {
		return err
	}

	for _, r := range results {
		total++
		for _, v := range r.Elements {
			if err := personalAccessTokensFormat.Print(w, "\t",
				v.Host.ProjectName(),
				v.Struct.Name,
				v.Struct.Scopes,
				v.Struct.Active,
				v.Struct.ExpiresAt,
				r.Cached,
			); err != nil {
				return err
			}
		}

	}
	if err := totalFormat.Print(w, "\n", unique, total, len(wg.Errors())); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func listPersonalAccessTokens(h *client.Host, opt gitlab.ListPersonalAccessTokensOptions, wg *limiter.Limiter, data chan<- sort.Element[*gitlab.PersonalAccessToken], options ...gitlab.RequestOptionFunc) {
	defer wg.Done()

	for {
		wg.Lock()
		list, resp, err := h.Client.PersonalAccessTokens.ListPersonalAccessTokens(&opt, options...)
		if err != nil {
			wg.Error(h, err)
			wg.Unlock()
			return
		}
		wg.Unlock()

		for _, v := range list {
			data <- sort.Element[*gitlab.PersonalAccessToken]{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}
}
