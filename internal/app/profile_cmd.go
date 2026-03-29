package app

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"noto/internal/profile"
	"noto/internal/store"
)

func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles",
	}
	cmd.AddCommand(profileCreateCmd())
	cmd.AddCommand(profileListCmd())
	cmd.AddCommand(profileSelectCmd())
	cmd.AddCommand(profileRenameCmd())
	cmd.AddCommand(profileDeleteCmd())
	return cmd
}

func profileCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "profile: close db: %v\n", err)
				}
			}()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Create(context.Background(), name)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(os.Stdout, "Created profile %q (slug: %s)\n", p.Name, p.Slug); err != nil {
				return err
			}
			return nil
		},
	}
}

func profileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "profile: close db: %v\n", err)
				}
			}()
			svc := profile.NewService(store.NewProfileRepo(db))
			profiles, err := svc.List(context.Background())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "NAME\tSLUG\tDEFAULT"); err != nil {
				return err
			}
			for _, p := range profiles {
				def := ""
				label := p.Name
				if p.IsDefault {
					def = "*"
				}
				if p.Name != p.Slug {
					label = fmt.Sprintf("%s (%s)", p.Name, p.Slug)
				}
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", label, p.Slug, def); err != nil {
					return err
				}
			}
			return w.Flush()
		},
	}
}

func profileSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <name>",
		Short: "Set the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "profile: close db: %v\n", err)
				}
			}()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Select(context.Background(), args[0])
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(os.Stdout, "Active profile: %q\n", p.Name); err != nil {
				return err
			}
			return nil
		},
	}
}

func profileRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "profile: close db: %v\n", err)
				}
			}()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Rename(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(os.Stdout, "Renamed to %q\n", p.Name); err != nil {
				return err
			}
			return nil
		},
	}
}

func profileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile (irreversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openGlobalDB()
			if err != nil {
				return err
			}
			defer func() {
				if err := db.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "profile: close db: %v\n", err)
				}
			}()
			repo := store.NewProfileRepo(db)
			svc := profile.NewService(repo)
			flow := profile.NewDeleteFlow(svc)
			return flow.Run(context.Background(), args[0], os.Stdout, os.Stdin)
		},
	}
}
