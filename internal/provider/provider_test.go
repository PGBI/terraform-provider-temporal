package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	testProviderConfig = `
provider "temporal" {
  address   = "localhost:7233"
  namespace = "default"
}
`
)

var (
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"temporal": providerserver.NewProtocol6WithError(New("test")()),
	}
)
