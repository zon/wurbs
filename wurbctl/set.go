package main

// SetCmd is the top-level `set` command group.
type SetCmd struct {
	Config SetConfigCmd `cmd:"" name:"config" help:"Configure postgres, OIDC settings, and generate k8s configmap and secret."`
	Admin  SetAdminCmd  `cmd:"" name:"admin" help:"Create or update an admin user and generate their client credential keys."`
}
