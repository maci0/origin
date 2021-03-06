package sa

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/serviceaccounts"
)

const (
	GetServiceAccountTokenRecommendedName = "get-token"

	getServiceAccountTokenShort = `Get a token assigned to a service account.`

	getServiceAccountTokenLong = `
Get a token assigned to a service account.

If the service account has multiple tokens, the first token found will be returned.

Service account API tokens are used by service accounts to authenticate to the API.
Client actions using a service account token will be executed as if the service account
itself were making the actions.
`

	getServiceAccountTokenUsage = `%s SA-NAME`

	getServiceAccountTokenExamples = `  # Get the service account token from service account 'default'
  %[1]s 'default'
`
)

type GetServiceAccountTokenOptions struct {
	SAName        string
	SAClient      unversioned.ServiceAccountsInterface
	SecretsClient unversioned.SecretsInterface

	Out io.Writer
	Err io.Writer
}

func NewCommandGetServiceAccountToken(name, fullname string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &GetServiceAccountTokenOptions{
		Out: out,
		Err: os.Stderr,
	}

	getServiceAccountTokenCommand := &cobra.Command{
		Use:     fmt.Sprintf(getServiceAccountTokenUsage, name),
		Short:   getServiceAccountTokenShort,
		Long:    getServiceAccountTokenLong,
		Example: fmt.Sprintf(getServiceAccountTokenExamples, fullname),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Complete(args, f, cmd))

			cmdutil.CheckErr(options.Validate())

			cmdutil.CheckErr(options.Run())
		},
	}

	return getServiceAccountTokenCommand
}

func (o *GetServiceAccountTokenOptions) Complete(args []string, f *clientcmd.Factory, cmd *cobra.Command) error {
	if len(args) != 1 {
		return cmdutil.UsageError(cmd, fmt.Sprintf("expected one service account name as an argument, got %q", args))
	}

	o.SAName = args[0]

	client, err := f.Client()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.SAClient = client.ServiceAccounts(namespace)
	o.SecretsClient = client.Secrets(namespace)
	return nil
}

func (o *GetServiceAccountTokenOptions) Validate() error {
	if o.SAName == "" {
		return errors.New("service account name cannot be empty")
	}

	if o.SAClient == nil || o.SecretsClient == nil {
		return errors.New("API clients must not be nil in order to create a new service account token")
	}

	if o.Out == nil || o.Err == nil {
		return errors.New("cannot proceed if output or error writers are nil")
	}

	return nil
}

func (o *GetServiceAccountTokenOptions) Run() error {
	serviceAccount, err := o.SAClient.Get(o.SAName)
	if err != nil {
		return err
	}

	for _, reference := range serviceAccount.Secrets {
		secret, err := o.SecretsClient.Get(reference.Name)
		if err != nil {
			continue
		}

		if serviceaccounts.IsValidServiceAccountToken(serviceAccount, secret) {
			token, exists := secret.Data[kapi.ServiceAccountTokenKey]
			if !exists {
				return fmt.Errorf("service account token %q for service account %q did not contain token data", secret.Name, serviceAccount.Name)
			}

			fmt.Fprintf(o.Out, string(token))
			if util.IsTerminalWriter(o.Out) {
				// pretty-print for a TTY
				fmt.Fprintf(o.Out, "\n")
			}
			return nil
		}
	}
	return fmt.Errorf("could not find a service account token for service account %q", serviceAccount.Name)
}
