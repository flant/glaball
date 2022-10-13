package users

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort"
	"github.com/flant/glaball/pkg/util"

	"github.com/flant/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	createOpt = gitlab.CreateUserOptions{}
)

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create user",
		Long: `Creates a new user. Note only administrators can create new users.
Either --password, --reset_password, or --force_random_password must be specified.
If --reset_password and --force_random_password are both false, then --password is required.

--force_random_password and --reset_password take priority over --password.
In addition, --reset_password and --force_random_password can be used together.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Create()
		},
	}

	// CreateUserOptions
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Email), "email", "Email.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Password), "password", "Password.")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.ResetPassword), "reset_password",
		"Send user password reset link - true or false (default).")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.ForceRandomPassword), "force_random_password",
		"Set user password to a random value - true or false (default).")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Username), "username", "Username.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Name), "name", "Name.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Skype), "skype", "Skype ID.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Linkedin), "linkedin", "LinkedIn.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Twitter), "twitter", "Twitter account.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.WebsiteURL), "website_url", "Website URL.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Organization), "organization", "Organization name.")
	cmd.Flags().Var(util.NewIntPtrValue(&createOpt.ProjectsLimit), "projects_limit",
		"Number of projects user can create.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.ExternUID), "extern_uid", "External UID.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Provider), "provider", "External provider name.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Bio), "bio", "User's biography.")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Location), "location", "User's location.")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.Admin), "admin", "User is admin - true or false (default).")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.CanCreateGroup), "can_create_group",
		"User can create groups - true or false.")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.SkipConfirmation), "skip_confirmation",
		"Skip confirmation - true or false (default).")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.External), "external",
		"Flags the user as external - true or false. Default is false")
	cmd.Flags().Var(util.NewBoolPtrValue(&createOpt.PrivateProfile), "private_profile",
		"User’s profile is private - true, false (default), or null (is converted to false).")
	cmd.Flags().Var(util.NewStringPtrValue(&createOpt.Note), "note", "Admin notes for this user.")

	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("name")

	return cmd
}

func Create() error {
	if createOpt.Password == nil &&
		(createOpt.ResetPassword == nil || !*createOpt.ResetPassword) &&
		(createOpt.ForceRandomPassword == nil || !*createOpt.ForceRandomPassword) {
		return fmt.Errorf("--password, --reset_password, --force_random_password are missing, at least one parameter must be provided")
	}

	util.AskUser(fmt.Sprintf("Do you really want to create user %q in %v ?",
		*createOpt.Username, common.Client.Hosts.Projects(common.Config.ShowAll)))

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		wg.Add(1)
		go createUser(h, createOpt, wg, data)
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"username"},
		StructType: gitlab.User{},
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tUSER\tHOSTS\tCACHED\n")

	total := 0
	for _, v := range results {
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(common.Config.ShowAll), v.Cached)
		total++
	}

	fmt.Fprintf(w, "Created: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}

func createUser(h *client.Host, opt gitlab.CreateUserOptions, wg *limiter.Limiter, data chan<- interface{},
	options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	user, resp, err := h.Client.Users.CreateUser(&opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: resp.Header.Get("X-From-Cache") == "1"}
}
