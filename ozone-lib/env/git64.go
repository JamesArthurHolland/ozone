package env

import (
	"encoding/base64"
	"fmt"
	"gopkg.in/src-d/go-git.v4"
	"github.com/JamesArthurHolland/ozone/ozone-lib/utils"
	"strings"
)

func FromGitDirBranchNameHash(varsParamMap map[string]string) (map[string]string, error) {
	dirPath := varsParamMap["ROOT_DIR"]
	if dirPath == "" {
		dirPath = "./"
	}

	varsMap := make(map[string]string)

	repo, err := git.PlainOpen(dirPath)
	if err != nil {
		return nil, err
	}

	reference, err := repo.Head()
	if err != nil {
		return nil, err
	}
	branchName := string(reference.Name())

	git64Hash := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(branchName))

	namespace := strings.ToLower(git64Hash)[:12]
	varsMap["NAMESPACE"] = namespace
	varsMap["SUBDOMAIN"] = fmt.Sprintf("%s.", namespace)

	return varsMap, nil
}

func GitSubmoduleHash(varsParamMap map[string]string) (map[string]string, error) {
	for _, arg := range []string{
		"DOCKER_REGISTRY",
		"DIR",
		"SERVICE",
	} {
		if err := utils.ParamsOK("GitSubmoduleHash", arg, varsParamMap); err != nil {
			return nil, err
		}
	}
	dockerRegistry := varsParamMap["DOCKER_REGISTRY"]
	dirPath := varsParamMap["DIR"]
	serviceName := varsParamMap["SERVICE"]

	varsMap := make(map[string]string)

	repo, err := git.PlainOpen(dirPath)
	reference, err := repo.Head()
	if err != nil {
		return nil, err
	}
	submoduleHash := reference.Hash().String()

	varsMap["FULL_TAG"] = fmt.Sprintf("%s/%s:%s", dockerRegistry, serviceName, submoduleHash)
	return varsMap, nil
}