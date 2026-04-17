package models

// User represents a user (fetched from LDAP Manager service)
type User struct {
	UID          string   `json:"uid"`
	CN           string   `json:"cn"`
	SN           string   `json:"sn"`
	GivenName    string   `json:"givenName"`
	Mail         string   `json:"mail"`
	Department   string   `json:"department"`
	UIDNumber    int      `json:"uidNumber"`
	GIDNumber    int      `json:"gidNumber"`
	HomeDir      string   `json:"homeDirectory"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// GiteaRepository represents a repository from Gitea
type GiteaRepository struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	FullName      string          `json:"fullName"`
	Description   string          `json:"description"`
	Private       bool            `json:"private"`
	Fork          bool            `json:"fork"`
	HTMLURL       string          `json:"htmlUrl"`
	SSHURL        string          `json:"sshUrl"`
	CloneURL      string          `json:"cloneUrl"`
	DefaultBranch string          `json:"defaultBranch"`
	Language      string          `json:"language"`
	Stars         int             `json:"stars"`
	Forks         int             `json:"forks"`
	Size          int             `json:"size"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
	Owner         RepositoryOwner `json:"owner"`
}

// RepositoryOwner represents the owner of a repository
type RepositoryOwner struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"fullName"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

// RepositoryStats contains statistics about repositories
type RepositoryStats struct {
	TotalCount   int                    `json:"totalCount"`
	PrivateCount int                    `json:"privateCount"`
	PublicCount  int                    `json:"publicCount"`
	Languages    []LanguageDistribution `json:"languages"`
}

// LanguageDistribution represents language usage statistics
type LanguageDistribution struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status      string `json:"status"`
	Timestamp   int64  `json:"timestamp"`
	Gitea       bool   `json:"gitea"`
	LDAPManager bool   `json:"ldapManager"`
}

// Branch represents a git branch
type Branch struct {
	Name              string     `json:"name"`
	Commit            CommitMeta `json:"commit"`
	Protected         bool       `json:"protected"`
	RequiredApprovals int        `json:"requiredApprovals"`
}

// CommitMeta represents commit metadata
type CommitMeta struct {
	SHA     string `json:"sha"`
	URL     string `json:"url"`
	Created string `json:"created"`
}

// Commit represents a git commit
type Commit struct {
	SHA       string       `json:"sha"`
	URL       string       `json:"url"`
	Commit    CommitDetail `json:"commit"`
	Author    GitUser      `json:"author"`
	Committer GitUser      `json:"committer"`
}

// CommitDetail represents commit details
type CommitDetail struct {
	Message   string  `json:"message"`
	Tree      TreeRef `json:"tree"`
	Author    GitUser `json:"author"`
	Committer GitUser `json:"committer"`
}

// TreeRef represents a git tree reference
type TreeRef struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

// GitUser represents a git user
type GitUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

// Tag represents a git tag
type Tag struct {
	Name       string     `json:"name"`
	Message    string     `json:"message"`
	Commit     CommitMeta `json:"commit"`
	ZipballURL string     `json:"zipballUrl"`
	TarballURL string     `json:"tarballUrl"`
}

// CreateRepositoryInput represents input for creating a repository
type CreateRepositoryInput struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	AutoInit      bool   `json:"autoInit"`
	Gitignores    string `json:"gitignores"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"defaultBranch"`
}

// MigrateRepositoryInput represents input for migrating a repository
type MigrateRepositoryInput struct {
	CloneAddr    string `json:"cloneAddr"`
	RepoName     string `json:"repoName"`
	RepoOwner    string `json:"repoOwner"`
	Mirror       bool   `json:"mirror"`
	Private      bool   `json:"private"`
	Description  string `json:"description"`
	Wiki         bool   `json:"wiki"`
	Milestones   bool   `json:"milestones"`
	Labels       bool   `json:"labels"`
	Issues       bool   `json:"issues"`
	PullRequests bool   `json:"pullRequests"`
	Releases     bool   `json:"releases"`
	AuthUsername string `json:"authUsername"`
	AuthPassword string `json:"authPassword"`
	AuthToken    string `json:"authToken"`
	Service      string `json:"service"` // github, gitlab, gitea, gogs
}

// PullRequest represents a pull request
type PullRequest struct {
	ID           int64            `json:"id"`
	Number       int64            `json:"number"`
	State        string           `json:"state"`
	Title        string           `json:"title"`
	Body         string           `json:"body"`
	User         PRUser           `json:"user"`
	Head         PRBranchInfo     `json:"head"`
	Base         PRBranchInfo     `json:"base"`
	Mergeable    bool             `json:"mergeable"`
	Merged       bool             `json:"merged"`
	MergedAt     string           `json:"mergedAt"`
	MergedBy     *PRUser          `json:"mergedBy"`
	CreatedAt    string           `json:"createdAt"`
	UpdatedAt    string           `json:"updatedAt"`
	ClosedAt     string           `json:"closedAt"`
	DueDate      string           `json:"dueDate"`
	Assignees    []PRUser         `json:"assignees"`
	Labels       []PRLabel        `json:"labels"`
	Milestone    *PRMilestone     `json:"milestone"`
	Comments     int              `json:"comments"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	ChangedFiles int              `json:"changedFiles"`
	HTMLURL      string           `json:"htmlUrl"`
	DiffURL      string           `json:"diffUrl"`
	PatchURL     string           `json:"patchUrl"`
}

// PRBranchInfo represents branch information in a PR
type PRBranchInfo struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
}

// PRUser represents a user in PR context
type PRUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"fullName"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

// PRLabel represents a label
type PRLabel struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// PRMilestone represents a milestone
type PRMilestone struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	DueOn       string `json:"dueOn"`
}

// PRComment represents a comment on a PR
type PRComment struct {
	ID        int64  `json:"id"`
	User      PRUser `json:"user"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	HTMLURL   string `json:"htmlUrl"`
}

// PRReview represents a review on a PR
type PRReview struct {
	ID          int64  `json:"id"`
	User        PRUser `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"`
	CommitID    string `json:"commitId"`
	SubmittedAt string `json:"submittedAt"`
	HTMLURL     string `json:"htmlUrl"`
}

// PRFile represents a file changed in a PR
type PRFile struct {
	Filename    string `json:"filename"`
	Status      string `json:"status"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	Changes     int    `json:"changes"`
	PatchURL    string `json:"patchUrl"`
	RawURL      string `json:"rawUrl"`
	ContentsURL string `json:"contentsUrl"`
}
