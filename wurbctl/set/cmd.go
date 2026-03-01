package set

// Cmd is the top-level `set` command group.
type Cmd struct {
	Config ConfigCmd `cmd:"" name:"config" help:"Configure postgres, OIDC settings, and generate k8s configmap and secret."`
	Admin  AdminCmd  `cmd:"" name:"admin" help:"Create or update an admin user and generate their client credential keys."`
}
