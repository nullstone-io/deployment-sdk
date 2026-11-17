package lambda

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// versionsToPrune picks which published versions to delete so that only the newest `keep` remain.
// Published versions are numbered in increasing order; anything else (e.g. $LATEST) is never pruned.
// The result is ordered oldest first.
func versionsToPrune(versions []string, keep int) []string {
	if keep < 0 {
		keep = 0
	}
	numbered := make([]int, 0, len(versions))
	for _, v := range versions {
		if n, err := strconv.Atoi(v); err == nil {
			numbered = append(numbered, n)
		}
	}
	sort.Ints(numbered)
	if len(numbered) <= keep {
		return nil
	}
	stale := numbered[:len(numbered)-keep]
	result := make([]string, 0, len(stale))
	for _, n := range stale {
		result = append(result, strconv.Itoa(n))
	}
	return result
}

// PruneVersions deletes published versions of the function, keeping only the newest `keep`.
// Old versions hold on to their container image (or code) and, with SnapStart, are billed for their
// snapshot for as long as they exist.
//
// A version that an alias still points to is left in place: AWS refuses to delete it.
// It returns the versions that were deleted.
func PruneVersions(ctx context.Context, infra Outputs, keep int) ([]string, error) {
	λClient := lambda.NewFromConfig(infra.DeployerAwsConfig())

	all := make([]string, 0)
	paginator := lambda.NewListVersionsByFunctionPaginator(λClient, &lambda.ListVersionsByFunctionInput{
		FunctionName: aws.String(infra.FunctionName()),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing lambda versions: %w", err)
		}
		for _, fn := range page.Versions {
			all = append(all, aws.ToString(fn.Version))
		}
	}

	deleted := make([]string, 0)
	for _, version := range versionsToPrune(all, keep) {
		_, err := λClient.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
			FunctionName: aws.String(infra.FunctionName()),
			// Qualifier scopes the delete to this version; without it, AWS deletes the whole function.
			Qualifier: aws.String(version),
		})
		if err != nil {
			var conflict *types.ResourceConflictException
			if errors.As(err, &conflict) {
				// An alias still references this version
				continue
			}
			return deleted, fmt.Errorf("error deleting lambda version %s: %w", version, err)
		}
		deleted = append(deleted, version)
	}
	return deleted, nil
}
