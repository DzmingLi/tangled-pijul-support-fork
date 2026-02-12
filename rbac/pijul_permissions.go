package rbac

func pijulOwnerPolicies(member, domain, repo string) [][]string {
	return [][]string{
		{member, domain, repo, PijulRead},
		{member, domain, repo, PijulCreateDiscussion},
		{member, domain, repo, PijulEditDiscussion},
		{member, domain, repo, PijulTagDiscussion},
		{member, domain, repo, PijulApply},
		{member, domain, repo, PijulEditChannels},
		{member, domain, repo, PijulEditTags},
		{member, domain, repo, PijulEditPermissions},
	}
}

func pijulCollaboratorPolicies(collaborator, domain, repo string) [][]string {
	return [][]string{
		{collaborator, domain, repo, PijulRead},
		{collaborator, domain, repo, PijulCreateDiscussion},
		{collaborator, domain, repo, PijulEditDiscussion},
		{collaborator, domain, repo, PijulTagDiscussion},
		{collaborator, domain, repo, PijulApply},
		{collaborator, domain, repo, PijulEditChannels},
		{collaborator, domain, repo, PijulEditTags},
	}
}

func (e *Enforcer) AddPijulRepoPermissions(member, domain, repo string) error {
	if err := checkRepoFormat(repo); err != nil {
		return err
	}

	_, err := e.E.AddPolicies(pijulOwnerPolicies(member, domain, repo))
	return err
}

func (e *Enforcer) RemovePijulRepoPermissions(member, domain, repo string) error {
	if err := checkRepoFormat(repo); err != nil {
		return err
	}

	_, err := e.E.RemovePolicies(pijulOwnerPolicies(member, domain, repo))
	return err
}

func (e *Enforcer) AddPijulCollaboratorPermissions(collaborator, domain, repo string) error {
	if err := checkRepoFormat(repo); err != nil {
		return err
	}

	_, err := e.E.AddPolicies(pijulCollaboratorPolicies(collaborator, domain, repo))
	return err
}

func (e *Enforcer) RemovePijulCollaboratorPermissions(collaborator, domain, repo string) error {
	if err := checkRepoFormat(repo); err != nil {
		return err
	}

	_, err := e.E.RemovePolicies(pijulCollaboratorPolicies(collaborator, domain, repo))
	return err
}
