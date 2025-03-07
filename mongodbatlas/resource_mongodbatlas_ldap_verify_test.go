package mongodbatlas

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cast"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	matlas "go.mongodb.org/atlas/mongodbatlas"
)

func TestAccAdvRSLDAPVerify_basic(t *testing.T) {
	SkipTestExtCred(t)
	var (
		ldapVerify   matlas.LDAPConfiguration
		resourceName = "mongodbatlas_ldap_verify.test"
		orgID        = os.Getenv("MONGODB_ATLAS_ORG_ID")
		projectName  = acctest.RandomWithPrefix("test-acc")
		clusterName  = acctest.RandomWithPrefix("test-acc")
		hostname     = os.Getenv("MONGODB_ATLAS_LDAP_HOSTNAME")
		username     = os.Getenv("MONGODB_ATLAS_LDAP_USERNAME")
		password     = os.Getenv("MONGODB_ATLAS_LDAP_PASSWORD")
		port         = os.Getenv("MONGODB_ATLAS_LDAP_PORT")
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testCheckLDAP(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckMongoDBAtlasLDAPVerifyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMongoDBAtlasLDAPVerifyConfig(projectName, orgID, clusterName, hostname, username, password, cast.ToInt(port)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongoDBAtlasLDAPVerifyExists(resourceName, &ldapVerify),

					resource.TestCheckResourceAttrSet(resourceName, "project_id"),
					resource.TestCheckResourceAttrSet(resourceName, "hostname"),
					resource.TestCheckResourceAttrSet(resourceName, "bind_username"),
					resource.TestCheckResourceAttrSet(resourceName, "request_id"),
					resource.TestCheckResourceAttrSet(resourceName, "port"),
				),
			},
		},
	})
}

func TestAccAdvRSLDAPVerifyWithConfiguration_CACertificate(t *testing.T) {
	SkipTestExtCred(t)
	var (
		ldapVerify    matlas.LDAPConfiguration
		resourceName  = "mongodbatlas_ldap_verify.test"
		orgID         = os.Getenv("MONGODB_ATLAS_ORG_ID")
		projectName   = acctest.RandomWithPrefix("test-acc")
		clusterName   = acctest.RandomWithPrefix("test-acc")
		hostname      = os.Getenv("MONGODB_ATLAS_LDAP_HOSTNAME")
		username      = os.Getenv("MONGODB_ATLAS_LDAP_USERNAME")
		password      = os.Getenv("MONGODB_ATLAS_LDAP_PASSWORD")
		port          = os.Getenv("MONGODB_ATLAS_LDAP_PORT")
		caCertificate = os.Getenv("MONGODB_ATLAS_LDAP_CA_CERTIFICATE")
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testCheckLDAP(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckMongoDBAtlasLDAPVerifyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMongoDBAtlasLDAPVerifyWithConfigurationConfig(projectName, orgID, clusterName, hostname, username, password, caCertificate, cast.ToInt(port), true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongoDBAtlasLDAPVerifyExists(resourceName, &ldapVerify),

					resource.TestCheckResourceAttrSet(resourceName, "project_id"),
					resource.TestCheckResourceAttrSet(resourceName, "hostname"),
					resource.TestCheckResourceAttrSet(resourceName, "bind_username"),
					resource.TestCheckResourceAttrSet(resourceName, "request_id"),
					resource.TestCheckResourceAttrSet(resourceName, "port"),
					resource.TestCheckResourceAttr(resourceName, "status", "SUCCESS"),
					resource.TestCheckResourceAttr(resourceName, "validations.0.validation_type", "SERVER_SPECIFIED"),
					resource.TestCheckResourceAttr(resourceName, "validations.0.status", "OK"),
					resource.TestCheckResourceAttr(resourceName, "validations.1.validation_type", "CONNECT"),
					resource.TestCheckResourceAttr(resourceName, "validations.1.status", "OK"),
					resource.TestCheckResourceAttr(resourceName, "validations.2.validation_type", "AUTHENTICATE"),
					resource.TestCheckResourceAttr(resourceName, "validations.2.status", "OK"),
				),
			},
		},
	})
}

func TestAccAdvRSLDAPVerify_importBasic(t *testing.T) {
	SkipTestExtCred(t)
	var (
		ldapConf     = matlas.LDAPConfiguration{}
		resourceName = "mongodbatlas_ldap_verify.test"
		orgID        = os.Getenv("MONGODB_ATLAS_ORG_ID")
		projectName  = acctest.RandomWithPrefix("test-acc")
		clusterName  = acctest.RandomWithPrefix("test-acc")
		hostname     = os.Getenv("MONGODB_ATLAS_LDAP_HOSTNAME")
		username     = os.Getenv("MONGODB_ATLAS_LDAP_USERNAME")
		password     = os.Getenv("MONGODB_ATLAS_LDAP_PASSWORD")
		port         = os.Getenv("MONGODB_ATLAS_LDAP_PORT")
	)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testCheckLDAP(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckMongoDBAtlasLDAPVerifyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccMongoDBAtlasLDAPVerifyConfig(projectName, orgID, clusterName, hostname, username, password, cast.ToInt(port)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckMongoDBAtlasLDAPVerifyExists(resourceName, &ldapConf),

					resource.TestCheckResourceAttrSet(resourceName, "project_id"),
					resource.TestCheckResourceAttrSet(resourceName, "hostname"),
					resource.TestCheckResourceAttrSet(resourceName, "bind_username"),
					resource.TestCheckResourceAttrSet(resourceName, "request_id"),
					resource.TestCheckResourceAttrSet(resourceName, "port"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportStateIdFunc:       testAccCheckMongoDBAtlasLDAPVerifyImportStateIDFunc(resourceName),
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"project_id", "bind_password"},
			},
		},
	})
}

func testAccCheckMongoDBAtlasLDAPVerifyExists(resourceName string, ldapConf *matlas.LDAPConfiguration) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := testAccProvider.Meta().(*MongoDBClient).Atlas

		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("no ID is set")
		}

		ldapConfRes, _, err := conn.LDAPConfigurations.GetStatus(context.Background(), rs.Primary.Attributes["project_id"], rs.Primary.Attributes["request_id"])
		if err != nil {
			return fmt.Errorf("ldapVerify (%s) does not exist", rs.Primary.ID)
		}

		ldapConf = ldapConfRes

		return nil
	}
}

func testAccCheckMongoDBAtlasLDAPVerifyDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*MongoDBClient).Atlas

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mongodbatlas_ldap_verify" {
			continue
		}

		_, _, err := conn.LDAPConfigurations.GetStatus(context.Background(), rs.Primary.Attributes["project_id"], rs.Primary.Attributes["request_id"])
		if err == nil {
			return fmt.Errorf("ldapVerify (%s) still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckMongoDBAtlasLDAPVerifyImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("not found: %s", resourceName)
		}

		return fmt.Sprintf("%s-%s", rs.Primary.Attributes["project_id"], rs.Primary.Attributes["request_id"]), nil
	}
}

func testAccMongoDBAtlasLDAPVerifyConfig(projectName, orgID, clusterName, hostname, username, password string, port int) string {
	return fmt.Sprintf(`
		resource "mongodbatlas_project" "test" {
			name   = "%[1]s"
			org_id = "%[2]s"
		}

		resource "mongodbatlas_cluster" "test" {
			project_id   = mongodbatlas_project.test.id
			name         = "%[3]s"
			
			// Provider Settings "block"
			provider_name               = "AWS"
			provider_region_name        = "US_EAST_2"
			provider_instance_size_name = "M10"
			provider_backup_enabled     = true //enable cloud provider snapshots
		}

		resource "mongodbatlas_ldap_verify" "test" {
			project_id               =  mongodbatlas_project.test.id
			hostname 				 = "%[4]s"
			port                     =  %[7]d
			bind_username            = "%[5]s"
			bind_password            = "%[6]s"
			depends_on = ["mongodbatlas_cluster.test"]
		}`, projectName, orgID, clusterName, hostname, username, password, port)
}

func testAccMongoDBAtlasLDAPVerifyWithConfigurationConfig(projectName, orgID, clusterName, hostname, username, password, caCertificate string, port int, authEnabled bool) string {
	return fmt.Sprintf(`
		resource "mongodbatlas_project" "test" {
			name   = "%[1]s"
			org_id = "%[2]s"
		}

		resource "mongodbatlas_cluster" "test" {
			project_id   = mongodbatlas_project.test.id
			name         = "%[3]s"
			
			// Provider Settings "block"
			provider_name               = "AWS"
			provider_region_name        = "US_EAST_2"
			provider_instance_size_name = "M10"
			provider_backup_enabled     = true //enable cloud provider snapshots
		}

		resource "mongodbatlas_ldap_verify" "test" {
			project_id                  = mongodbatlas_project.test.id
			hostname = "%[4]s"
			port                     = %[7]d
			bind_username                     = "%[5]s"
			bind_password                     = "%[6]s"
			ca_certificate = <<-EOF
%[9]s
			EOF
			depends_on = [mongodbatlas_cluster.test]
		}

		resource "mongodbatlas_ldap_configuration" "test" {
			project_id                  = mongodbatlas_project.test.id
			authentication_enabled                = %[8]t
			authorization_enabled                = false
			hostname = "%[4]s"
			port                     = %[7]d
			bind_username                     = "%[5]s"
			bind_password                     = "%[6]s"
			ca_certificate = <<-EOF
%[9]s
			EOF
			depends_on = [mongodbatlas_ldap_verify.test]
		}`, projectName, orgID, clusterName, hostname, username, password, port, authEnabled, caCertificate)
}
