package set

// Cmd is the top-level `set` command group.
type Cmd struct {
	Config ConfigCmd `cmd:"" name:"config" help:"Configure postgres, OIDC settings, and generate k8s configmap and secret."`
}
