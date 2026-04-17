package main

import (
	"fmt"
	"log"
	"os"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	ldapURL := env("LDAP_URL", "ldap://openldap1.dev-platform.svc.cluster.local:389")
	baseDN := env("LDAP_BASE_DN", "dc=devplatform,dc=local")
	adminDN := env("LDAP_BIND_DN", "cn=admin,dc=devplatform,dc=local")
	adminPW := env("LDAP_ADMIN_PASSWORD", "admin123")
	configDN := env("LDAP_CONFIG_DN", "cn=admin,cn=config")
	configPW := env("LDAP_CONFIG_PASSWORD", "config123")

	// ─── Wait for OpenLDAP ───
	fmt.Println("── Waiting for OpenLDAP to be ready ──")
	maxRetries := 60
	for i := 1; i <= maxRetries; i++ {
		c, err := ldap.DialURL(ldapURL)
		if err == nil {
			if bindErr := c.Bind(adminDN, adminPW); bindErr == nil {
				fmt.Printf("OpenLDAP ready after %d attempts\n", i)
				c.Close()
				break
			}
			c.Close()
		}
		if i == maxRetries {
			log.Fatalf("OpenLDAP not ready after %d attempts, giving up", maxRetries)
		}
		fmt.Printf("Waiting for OpenLDAP (attempt %d/%d)...\n", i, maxRetries)
		time.Sleep(5 * time.Second)
	}

	// ─── Register custom schema (githubRepository attribute) ───
	fmt.Println("\n── Registering custom LDAP schema ──")
	configConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		log.Fatalf("Failed to connect for schema: %v", err)
	}

	err = configConn.Bind(configDN, configPW)
	if err != nil {
		log.Printf("Warning: Failed to bind as config admin: %v (schema registration skipped)", err)
	} else {
		fmt.Println("Connected to cn=config as admin")

		sr, err := configConn.Search(ldap.NewSearchRequest(
			"cn=schema,cn=config", ldap.ScopeSingleLevel,
			ldap.NeverDerefAliases, 0, 0, false,
			"(cn=*devplatform)", []string{"cn"}, nil,
		))
		if err == nil && len(sr.Entries) > 0 {
			fmt.Println("Custom schema (devplatform) already exists, skipping")
		} else {
			addSchemaReq := ldap.NewAddRequest("cn=devplatform,cn=schema,cn=config", nil)
			addSchemaReq.Attribute("objectClass", []string{"olcSchemaConfig"})
			addSchemaReq.Attribute("cn", []string{"devplatform"})
			addSchemaReq.Attribute("olcAttributeTypes", []string{
				"( 1.3.6.1.4.1.99999.1.1 NAME 'githubRepository' " +
					"DESC 'Repository URL (GitHub/Gitea)' " +
					"EQUALITY caseIgnoreMatch " +
					"SUBSTR caseIgnoreSubstringsMatch " +
					"SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			})
			if err := configConn.Add(addSchemaReq); err != nil {
				log.Printf("Warning: Failed to register custom schema: %v", err)
			} else {
				fmt.Println("Registered custom schema (githubRepository attribute)")
			}
		}
	}
	configConn.Close()

	// ─── Connect as data admin ───
	fmt.Println("\n── Initializing LDAP data ──")
	conn, err := ldap.DialURL(ldapURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if err := conn.Bind(adminDN, adminPW); err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}
	fmt.Println("Connected to LDAP as admin")

	// ─── Create base OUs ───
	fmt.Println("\n── Creating OUs ──")
	for _, ou := range []struct{ name, desc string }{
		{"users", "Platform users"},
		{"groups", "Platform groups"},
		{"departments", "Platform departments"},
	} {
		addReq := ldap.NewAddRequest(fmt.Sprintf("ou=%s,%s", ou.name, baseDN), nil)
		addReq.Attribute("objectClass", []string{"organizationalUnit"})
		addReq.Attribute("ou", []string{ou.name})
		addReq.Attribute("description", []string{ou.desc})
		if err := conn.Add(addReq); err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
				fmt.Printf("  OU %s already exists\n", ou.name)
			} else {
				log.Printf("  Failed to create OU %s: %v", ou.name, err)
			}
		} else {
			fmt.Printf("  Created OU: %s\n", ou.name)
		}
	}

	// ─── Create departments ───
	fmt.Println("\n── Creating departments ──")
	for _, dept := range []struct{ name, desc string }{
		{"engineering", "Engineering Department"},
		{"devops", "DevOps Department"},
		{"datascience", "Data Science Department"},
		{"frontend", "Frontend Department"},
	} {
		deptDN := fmt.Sprintf("ou=%s,ou=departments,%s", dept.name, baseDN)
		addReq := ldap.NewAddRequest(deptDN, nil)
		addReq.Attribute("objectClass", []string{"organizationalUnit", "extensibleObject"})
		addReq.Attribute("ou", []string{dept.name})
		addReq.Attribute("description", []string{dept.desc})
		if err := conn.Add(addReq); err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
				fmt.Printf("  Department %s already exists\n", dept.name)
			} else {
				log.Printf("  Failed to create department %s: %v", dept.name, err)
			}
		} else {
			fmt.Printf("  Created department: %s\n", dept.name)
		}
	}

	// ─── Create groups ───
	fmt.Println("\n── Creating groups ──")
	for _, g := range []struct{ cn, gid, desc string }{
		{"developers", "10000", "All developers"},
		{"admins", "10001", "Platform administrators"},
		{"leads", "10002", "Team leads"},
	} {
		groupDN := fmt.Sprintf("cn=%s,ou=groups,%s", g.cn, baseDN)
		addReq := ldap.NewAddRequest(groupDN, nil)
		addReq.Attribute("objectClass", []string{"groupOfNames", "extensibleObject"})
		addReq.Attribute("cn", []string{g.cn})
		addReq.Attribute("gidNumber", []string{g.gid})
		addReq.Attribute("member", []string{fmt.Sprintf("cn=admin,%s", baseDN)})
		addReq.Attribute("description", []string{g.desc})
		if err := conn.Add(addReq); err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
				fmt.Printf("  Group %s already exists\n", g.cn)
			} else {
				log.Printf("  Failed to create group %s: %v", g.cn, err)
			}
		} else {
			fmt.Printf("  Created group: %s\n", g.cn)
		}
	}

	// ─── Create users ───
	fmt.Println("\n── Creating users ──")
	type user struct {
		uid, cn, sn, given, mail, dept, uidNum, gidNum string
		repos                                          []string
	}
	users := []user{
		{"john.doe", "John Doe", "Doe", "John", "john.doe@devplatform.local", "engineering", "10000", "10000",
			[]string{"john.doe/getting-started-todo-app"}},
		{"jane.smith", "Jane Smith", "Smith", "Jane", "jane.smith@devplatform.local", "engineering", "10001", "10000",
			[]string{"jane.smith/microservice-template"}},
		{"bob.wilson", "Bob Wilson", "Wilson", "Bob", "bob.wilson@devplatform.local", "devops", "10002", "10000",
			[]string{"bob.wilson/infra-tools"}},
		{"alice.chen", "Alice Chen", "Chen", "Alice", "alice.chen@devplatform.local", "datascience", "10003", "10000",
			[]string{"alice.chen/ml-pipeline"}},
		{"mike.jones", "Mike Jones", "Jones", "Mike", "mike.jones@devplatform.local", "frontend", "10004", "10000",
			[]string{"mike.jones/react-dashboard"}},
	}

	for _, u := range users {
		userDN := fmt.Sprintf("uid=%s,ou=users,%s", u.uid, baseDN)

		// Check existence
		sr, searchErr := conn.Search(ldap.NewSearchRequest(
			userDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
			"(objectClass=*)", []string{"objectClass"}, nil,
		))
		exists := searchErr == nil && len(sr.Entries) > 0

		if !exists {
			addReq := ldap.NewAddRequest(userDN, nil)
			addReq.Attribute("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
			addReq.Attribute("uid", []string{u.uid})
			addReq.Attribute("cn", []string{u.cn})
			addReq.Attribute("sn", []string{u.sn})
			addReq.Attribute("givenName", []string{u.given})
			addReq.Attribute("mail", []string{u.mail})
			addReq.Attribute("departmentNumber", []string{u.dept})
			addReq.Attribute("uidNumber", []string{u.uidNum})
			addReq.Attribute("gidNumber", []string{u.gidNum})
			addReq.Attribute("homeDirectory", []string{fmt.Sprintf("/home/%s", u.uid)})
			addReq.Attribute("userPassword", []string{"password123"})
			addReq.Attribute("loginShell", []string{"/bin/bash"})
			if len(u.repos) > 0 {
				addReq.Attribute("githubRepository", u.repos)
			}
			if err := conn.Add(addReq); err != nil {
				log.Printf("  Failed to create user %s: %v", u.uid, err)
			} else {
				fmt.Printf("  Created user: %s (%s)\n", u.uid, u.dept)
			}
		} else {
			fmt.Printf("  User %s already exists, updating...\n", u.uid)
			modReq := ldap.NewModifyRequest(userDN, nil)
			modReq.Replace("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
			modReq.Replace("departmentNumber", []string{u.dept})
			if len(u.repos) > 0 {
				modReq.Replace("githubRepository", u.repos)
			}
			if err := conn.Modify(modReq); err != nil {
				log.Printf("  Failed to update user %s: %v", u.uid, err)
			} else {
				fmt.Printf("  Updated user: %s\n", u.uid)
			}
		}
	}

	// ─── Add users to groups ───
	fmt.Println("\n── Adding users to groups ──")
	for _, u := range users {
		memberDN := fmt.Sprintf("uid=%s,ou=users,%s", u.uid, baseDN)
		groupDN := fmt.Sprintf("cn=developers,ou=groups,%s", baseDN)
		modReq := ldap.NewModifyRequest(groupDN, nil)
		modReq.Add("member", []string{memberDN})
		if err := conn.Modify(modReq); err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultAttributeOrValueExists) {
				fmt.Printf("  %s already in developers\n", u.uid)
			} else {
				log.Printf("  Failed to add %s to developers: %v", u.uid, err)
			}
		} else {
			fmt.Printf("  Added %s to developers\n", u.uid)
		}
	}

	// jane.smith → leads
	modReq := ldap.NewModifyRequest(fmt.Sprintf("cn=leads,ou=groups,%s", baseDN), nil)
	modReq.Add("member", []string{fmt.Sprintf("uid=jane.smith,ou=users,%s", baseDN)})
	if err := conn.Modify(modReq); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultAttributeOrValueExists) {
			fmt.Println("  jane.smith already in leads")
		} else {
			log.Printf("  Failed to add jane.smith to leads: %v", err)
		}
	} else {
		fmt.Println("  Added jane.smith to leads")
	}

	// ─── Verify ───
	fmt.Println("\n── Verification ──")
	userResult, err := conn.Search(ldap.NewSearchRequest(
		fmt.Sprintf("ou=users,%s", baseDN), ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=inetOrgPerson)",
		[]string{"uid", "cn", "mail", "departmentNumber", "githubRepository"}, nil,
	))
	if err != nil {
		log.Printf("Failed to verify users: %v", err)
	} else {
		fmt.Printf("Total users: %d\n", len(userResult.Entries))
		for _, entry := range userResult.Entries {
			fmt.Printf("  %s | %s | %s | repos: %v\n",
				entry.GetAttributeValue("uid"),
				entry.GetAttributeValue("mail"),
				entry.GetAttributeValue("departmentNumber"),
				entry.GetAttributeValues("githubRepository"),
			)
		}
	}

	fmt.Println("\nLDAP Initialization Complete!")
}
