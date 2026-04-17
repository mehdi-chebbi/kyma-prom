package main

import (
	"fmt"
	"log"

	ldap "github.com/go-ldap/ldap/v3"
)

func main() {
	// ─── Step 0: Register custom schema (githubRepository attribute) ───
	fmt.Println("── Registering custom LDAP schema ──")

	configConn, err := ldap.DialURL("ldap://localhost:30000")
	if err != nil {
		log.Fatalf("Failed to connect for schema: %v", err)
	}

	err = configConn.Bind("cn=admin,cn=config", "config123")
	if err != nil {
		log.Fatalf("Failed to bind as config admin: %v", err)
	}

	fmt.Println("✓ Connected to cn=config as admin")

	// Check if custom schema already exists
	searchReq := ldap.NewSearchRequest(
		"cn=schema,cn=config",
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases, 0, 0, false,
		"(cn=*devplatform)",
		[]string{"cn"},
		nil,
	)

	sr, err := configConn.Search(searchReq)
	if err == nil && len(sr.Entries) > 0 {
		fmt.Println("⚠ Custom schema (devplatform) already exists, skipping")
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

		err = configConn.Add(addSchemaReq)
		if err != nil {
			log.Fatalf("Failed to register custom schema: %v", err)
		}
		fmt.Println("✓ Registered custom schema (githubRepository attribute)")
	}

	configConn.Close()

	// ─── Step 1: Connect as data admin ───
	fmt.Println("\n── Initializing LDAP data ──")

	conn, err := ldap.DialURL("ldap://localhost:30000")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	err = conn.Bind("cn=admin,dc=devplatform,dc=local", "admin123")
	if err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}

	fmt.Println("✓ Connected to LDAP as admin")

	// ─── Step 2: Create base OUs ───
	ous := []struct {
		dn   string
		ou   string
		desc string
	}{
		{"ou=users,dc=devplatform,dc=local", "users", "Users"},
		{"ou=groups,dc=devplatform,dc=local", "groups", "Groups"},
		{"ou=departments,dc=devplatform,dc=local", "departments", "Departments"},
	}

	for _, ou := range ous {
		addReq := ldap.NewAddRequest(ou.dn, nil)
		addReq.Attribute("objectClass", []string{"organizationalUnit"})
		addReq.Attribute("ou", []string{ou.ou})
		addReq.Attribute("description", []string{ou.desc})

		err = conn.Add(addReq)
		if err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
				fmt.Printf("⚠ OU %s already exists\n", ou.ou)
			} else {
				log.Printf("Failed to create OU %s: %v", ou.ou, err)
			}
		} else {
			fmt.Printf("✓ Created OU: %s\n", ou.ou)
		}
	}

	// ─── Step 3: Create user john.doe (or update if exists) ───
	userDN := "uid=john.doe,ou=users,dc=devplatform,dc=local"

	// Check if user already exists
	userSearch := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"objectClass"},
		nil,
	)

	userResult, userErr := conn.Search(userSearch)
	userExists := userErr == nil && len(userResult.Entries) > 0

	if !userExists {
		// Create user with extensibleObject from the start
		addUserReq := ldap.NewAddRequest(userDN, nil)
		addUserReq.Attribute("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
		addUserReq.Attribute("uid", []string{"john.doe"})
		addUserReq.Attribute("cn", []string{"John Doe"})
		addUserReq.Attribute("sn", []string{"Doe"})
		addUserReq.Attribute("givenName", []string{"John"})
		addUserReq.Attribute("mail", []string{"john.doe@devplatform.local"})
		addUserReq.Attribute("uidNumber", []string{"10001"})
		addUserReq.Attribute("gidNumber", []string{"10001"})
		addUserReq.Attribute("homeDirectory", []string{"/home/john.doe"})
		addUserReq.Attribute("userPassword", []string{"password123"})
		addUserReq.Attribute("departmentNumber", []string{"engineering"})
		addUserReq.Attribute("githubRepository", []string{
			"john.doe/getting-started-todo-app",
		})

		err = conn.Add(addUserReq)
		if err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		fmt.Println("✓ Created user: john.doe (with extensibleObject + repos)")
	} else {
		fmt.Println("⚠ User john.doe already exists — updating objectClass + repos")

		// Add extensibleObject to objectClass if missing
		modReq := ldap.NewModifyRequest(userDN, nil)
		modReq.Replace("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
		modReq.Replace("departmentNumber", []string{"engineering"})
		modReq.Replace("githubRepository", []string{
			"john.doe/getting-started-todo-app",
		})

		err = conn.Modify(modReq)
		if err != nil {
			log.Printf("Failed to update john.doe: %v", err)
			fmt.Println("  Trying attribute-by-attribute...")

			// Try adding extensibleObject first
			modOC := ldap.NewModifyRequest(userDN, nil)
			modOC.Replace("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
			if ocErr := conn.Modify(modOC); ocErr != nil {
				log.Printf("  Failed to add extensibleObject: %v", ocErr)
			} else {
				fmt.Println("  ✓ Added extensibleObject to john.doe")
			}

			// Now add githubRepository
			modRepo := ldap.NewModifyRequest(userDN, nil)
			modRepo.Replace("githubRepository", []string{
				"john.doe/getting-started-todo-app",
			})
			if repoErr := conn.Modify(modRepo); repoErr != nil {
				log.Printf("  Failed to add githubRepository: %v", repoErr)
			} else {
				fmt.Println("  ✓ Added githubRepository to john.doe")
			}

			// Add departmentNumber
			modDept := ldap.NewModifyRequest(userDN, nil)
			modDept.Replace("departmentNumber", []string{"engineering"})
			if deptErr := conn.Modify(modDept); deptErr != nil {
				log.Printf("  Failed to add departmentNumber: %v", deptErr)
			} else {
				fmt.Println("  ✓ Added departmentNumber to john.doe")
			}
		} else {
			fmt.Println("✓ Updated john.doe with extensibleObject + repos + department")
		}
	}

	// ─── Step 4: Create departments ───
	departments := []struct {
		name string
		desc string
	}{
		{"engineering", "Engineering Department"},
		{"devops", "DevOps Department"},
		{"datascience", "Data Science Department"},
		{"frontend", "Frontend Department"},
	}

	for _, dept := range departments {
		deptDN := fmt.Sprintf("ou=%s,ou=departments,dc=devplatform,dc=local", dept.name)
		addDeptReq := ldap.NewAddRequest(deptDN, nil)
		addDeptReq.Attribute("objectClass", []string{"organizationalUnit"})
		addDeptReq.Attribute("ou", []string{dept.name})
		addDeptReq.Attribute("description", []string{dept.desc})

		err = conn.Add(addDeptReq)
		if err != nil {
			if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
				fmt.Printf("⚠ Department %s already exists\n", dept.name)
			} else {
				log.Printf("Failed to create department %s: %v", dept.name, err)
			}
		} else {
			fmt.Printf("✓ Created department: %s\n", dept.name)
		}
	}

	// ─── Step 5: Verify john.doe ───
	fmt.Println("\n── Verifying john.doe ──")
	verifyReq := ldap.NewSearchRequest(
		userDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"uid", "cn", "mail", "objectClass", "departmentNumber", "githubRepository"},
		nil,
	)

	verifyResult, verifyErr := conn.Search(verifyReq)
	if verifyErr != nil {
		log.Printf("Failed to verify john.doe: %v", verifyErr)
	} else if len(verifyResult.Entries) > 0 {
		entry := verifyResult.Entries[0]
		fmt.Printf("  DN: %s\n", entry.DN)
		fmt.Printf("  objectClass: %v\n", entry.GetAttributeValues("objectClass"))
		fmt.Printf("  uid: %s\n", entry.GetAttributeValue("uid"))
		fmt.Printf("  cn: %s\n", entry.GetAttributeValue("cn"))
		fmt.Printf("  mail: %s\n", entry.GetAttributeValue("mail"))
		fmt.Printf("  departmentNumber: %s\n", entry.GetAttributeValue("departmentNumber"))
		fmt.Printf("  githubRepository: %v\n", entry.GetAttributeValues("githubRepository"))
	}

	fmt.Println("\n✅ LDAP Initialization Complete!")
	fmt.Println("\nLogin Credentials:")
	fmt.Println("  Username: john.doe")
	fmt.Println("  Password: password123")
	fmt.Println("  Department: engineering")
	fmt.Println("  Repositories: john.doe/getting-started-todo-app")
}
