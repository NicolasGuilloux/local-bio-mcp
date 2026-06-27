package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLoginCmd(a *app) *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with your local.bio account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if email == "" {
				email = os.Getenv("LOCALBIO_EMAIL")
			}
			if password == "" {
				password = os.Getenv("LOCALBIO_PASSWORD")
			}
			if email == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Email: ")
				if _, err := fmt.Fscanln(cmd.InOrStdin(), &email); err != nil {
					return fmt.Errorf("read email: %w", err)
				}
			}
			if password == "" {
				fmt.Fprint(cmd.OutOrStdout(), "Password: ")
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = strings.TrimSpace(string(b))
			}

			lr, err := a.client().Login(cmd.Context(), email, password)
			if err != nil {
				return err
			}
			token := lr.TokenValue()
			if token == "" {
				return fmt.Errorf("login succeeded but no token was found in the response "+
					"(fields: %s).\nRe-run with LOCALBIO_DEBUG=1 to inspect the raw payload "+
					"and please report it", lr.Fields())
			}
			a.cfg.Token = token
			a.cfg.Email = email
			if err := a.cfg.Save(); err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), map[string]any{"loggedIn": true, "email": email})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\n", email)
			return nil
		},
	}
	cmd.Flags().StringVarP(&email, "email", "e", "", "account email (or LOCALBIO_EMAIL)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "account password (or LOCALBIO_PASSWORD)")
	return cmd
}

func newLogoutCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and clear stored tokens",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.cfg.Token != "" {
				_ = a.client().Logout(cmd.Context()) // best effort
			}
			a.cfg.Clear()
			if err := a.cfg.Save(); err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), map[string]any{"loggedOut": true})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}

func newInfoCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show info about your account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.cfg.Token == "" {
				return fmt.Errorf("not logged in (run `localbio login`)")
			}
			acc, err := a.client().Me(cmd.Context())
			if err != nil {
				return err
			}
			if a.jsonOutput() {
				return emitJSON(cmd.OutOrStdout(), acc.Raw)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Email:    %s\n", acc.Email)
			if name := strings.TrimSpace(acc.FirstName + " " + acc.LastName); name != "" {
				fmt.Fprintf(out, "Name:     %s\n", name)
			}
			if acc.Phone != "" {
				fmt.Fprintf(out, "Phone:    %s\n", acc.Phone)
			}
			store := acc.StoreID
			if store == "" {
				store = a.cfg.StoreID
			}
			if store != "" {
				fmt.Fprintf(out, "Store:    %s\n", store)
			}
			return nil
		},
	}
}
