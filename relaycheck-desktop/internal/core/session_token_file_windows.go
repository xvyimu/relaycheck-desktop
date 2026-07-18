//go:build windows

package core

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

func currentProcessUserSID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	if user == nil || user.User.Sid == nil {
		return "", fmt.Errorf("current process token has no user SID")
	}
	return user.User.Sid.String(), nil
}

func secureSessionTokenFile(path string) error {
	return secureCurrentUserFile(path)
}

func secureInstanceKeyFile(path string) error {
	return secureCurrentUserFile(path)
}

func secureCurrentUserFile(path string) error {
	userSID, err := currentProcessUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString(
		fmt.Sprintf("D:P(A;;FA;;;%s)(A;;FA;;;SY)(A;;FA;;;BA)", userSID),
	)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}

func verifySessionTokenFileSecurity(path string) error {
	return verifyCurrentUserFileSecurity(path)
}

func verifyInstanceKeyFileSecurity(path string) error {
	return verifyCurrentUserFileSecurity(path)
}

func verifyCurrentUserFileSecurity(path string) error {
	userSID, err := currentProcessUserSID()
	if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	sddl := descriptor.String()
	for _, required := range []string{"D:P", ";;;" + userSID + ")", ";;;SY)", ";;;BA)"} {
		if !strings.Contains(sddl, required) {
			return fmt.Errorf("required ACL entry %q missing from %q", required, sddl)
		}
	}
	for _, forbidden := range []string{";;;WD)", ";;;BU)", ";;;AU)", ";;;AN)"} {
		if strings.Contains(sddl, forbidden) {
			return fmt.Errorf("broad ACL entry %q present in %q", forbidden, sddl)
		}
	}
	return nil
}
