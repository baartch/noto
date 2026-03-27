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
			defer db.Close()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Create(context.Background(), name)
			if err != nil {
				return err
			}
			fmt.Printf("Created profile %q (slug: %s)\n", p.Name, p.Slug)
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
			defer db.Close()
			svc := profile.NewService(store.NewProfileRepo(db))
			profiles, err := svc.List(context.Background())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSLUG\tDEFAULT")
			for _, p := range profiles {
				def := ""
				if p.IsDefault {
					def = "*"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Slug, def)
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
			defer db.Close()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Select(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Active profile: %q\n", p.Name)
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
			defer db.Close()
			svc := profile.NewService(store.NewProfileRepo(db))
			p, err := svc.Rename(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Renamed to %q\n", p.Name)
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
			defer db.Close()
			repo := store.NewProfileRepo(db)
			svc := profile.NewService(repo)
			flow := profile.NewDeleteFlow(svc)
			return flow.Run(context.Background(), args[0], os.Stdout, os.Stdin)
		},
	}
}
