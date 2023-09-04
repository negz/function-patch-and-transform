package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/negz/function-patch-and-transform/input/v1beta1"
)

func TestPatchApply(t *testing.T) {
	now := metav1.NewTime(time.Unix(0, 0))
	lpt := fake.ConnectionDetailsLastPublishedTimer{
		Time: &now,
	}

	errNotFound := func(path string) error {
		p := &fieldpath.Paved{}
		_, err := p.GetValue(path)
		return err
	}

	type args struct {
		patch v1beta1.Patch
		cp    *fake.Composite
		cd    *fake.Composed
		only  []v1beta1.PatchType
	}
	type want struct {
		cp  *fake.Composite
		cd  *fake.Composed
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidCompositeFieldPathPatch": {
			reason: "Should return error when required fields not passed to applyFromFieldPathPatch",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeFromCompositeFieldPath,
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
			},
			want: want{
				err: errors.Errorf(errFmtRequiredField, "FromFieldPath", v1beta1.PatchTypeFromCompositeFieldPath),
			},
		},
		"Invalidv1.PatchType": {
			reason: "Should return an error if an invalid patch type is specified",
			args: args{
				patch: v1beta1.Patch{
					Type: "invalid-patchtype",
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{Name: "cd"}},
			},
			want: want{
				err: errors.Errorf(errFmtInvalidPatchType, "invalid-patchtype"),
			},
		},
		"ValidCompositeFieldPathPatch": {
			reason: "Should correctly apply a CompositeFieldPathPatch with valid settings",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCompositeFieldPathPatchWithNilLastPublishTime": {
			reason: "Should correctly apply a CompositeFieldPathPatch with valid settings",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
				err: nil,
			},
		},
		"ValidCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, adds a field to each element of an array",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.name"),
					ToFieldPath:   pointer.String("objectMeta.ownerReferences[*].name"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "",
								APIVersion: "v1",
							},
							{
								Name:       "",
								APIVersion: "v1alpha1",
							},
						},
					},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
							{
								Name:       "test",
								APIVersion: "v1alpha1",
							},
						},
					},
				},
			},
		},
		"InvalidCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, throws an error if ToFieldPath cannot be expanded",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.name"),
					ToFieldPath:   pointer.String("objectMeta.ownerReferences[*].badField"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
						},
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtExpandingArrayFieldPaths, "objectMeta.ownerReferences[*].badField"),
			},
		},
		"MissingOptionalFieldPath": {
			reason: "A FromFieldPath patch should be a no-op when an optional fromFieldPath doesn't exist",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: nil,
			},
		},
		"MissingRequiredFieldPath": {
			reason: "A FromFieldPath patch should return an error when a required fromFieldPath doesn't exist",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("wat"),
					Policy: &v1beta1.PatchPolicy{
						FromFieldPath: func() *v1beta1.FromFieldPathPolicy {
							s := v1beta1.FromFieldPathPolicyRequired
							return &s
						}(),
					},
					ToFieldPath: pointer.String("wat"),
				},
				cp: &fake.Composite{
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: errNotFound("wat"),
			},
		},
		"FilterExcludeCompositeFieldPathPatch": {
			reason: "Should not apply the patch as the v1.PatchType is not present in filter.",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
				only: []v1beta1.PatchType{v1beta1.PatchTypePatchSet},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
				err: nil,
			},
		},
		"FilterIncludeCompositeFieldPathPatch": {
			reason: "Should apply the patch as the v1.PatchType is present in filter.",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{Name: "cd"},
				},
				only: []v1beta1.PatchType{v1beta1.PatchTypeFromCompositeFieldPath},
			},
			want: want{
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: nil,
			},
		},
		"DefaultToFieldCompositeFieldPathPatch": {
			reason: "Should correctly default the ToFieldPath value if not specified.",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: nil,
			},
		},
		"ValidToCompositeFieldPathPatch": {
			reason: "Should correctly apply a ToCompositeFieldPath patch with valid settings",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeToCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.labels"),
					ToFieldPath:   pointer.String("objectMeta.labels"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: nil,
			},
		},
		"ValidToCompositeFieldPathPatchWithWildcards": {
			reason: "When passed a wildcarded path, adds a field to each element of an array",
			args: args{
				patch: v1beta1.Patch{
					Type:          v1beta1.PatchTypeToCompositeFieldPath,
					FromFieldPath: pointer.String("objectMeta.name"),
					ToFieldPath:   pointer.String("objectMeta.ownerReferences[*].name"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "",
								APIVersion: "v1",
							},
							{
								Name:       "",
								APIVersion: "v1alpha1",
							},
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "test",
								APIVersion: "v1",
							},
							{
								Name:       "test",
								APIVersion: "v1alpha1",
							},
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
			},
		},
		"MissingCombineFromCompositeConfig": {
			reason: "Should return an error if Combine config is not passed",
			args: args{
				patch: v1beta1.Patch{
					Type:        v1beta1.PatchTypeCombineFromComposite,
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: errors.Errorf(errFmtRequiredField, "Combine", v1beta1.PatchTypeCombineFromComposite),
			},
		},
		"MissingCombineStrategyFromCompositeConfig": {
			reason: "Should return an error if Combine strategy config is not passed",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeCombineFromComposite,
					Combine: &v1beta1.Combine{
						Variables: []v1beta1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1beta1.CombineStrategyString,
					},
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: errors.Errorf(errFmtCombineConfigMissing, v1beta1.CombineStrategyString),
			},
		},
		"MissingCombineVariablesFromCompositeConfig": {
			reason: "Should return an error if no variables have been passed",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeCombineFromComposite,
					Combine: &v1beta1.Combine{
						Variables: []v1beta1.CombineVariable{},
						Strategy:  v1beta1.CombineStrategyString,
						String:    &v1beta1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: errors.New(errCombineRequiresVariables),
			},
		},
		"NoOpOptionalInputFieldFromCompositeConfig": {
			// Note: OptionalFieldPathNotFound is tested below, but we want to
			// test that we abort the patch if _any_ of our source fields are
			// not available.
			reason: "Should return no error and not apply patch if an optional variable is missing",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeCombineFromComposite,
					Combine: &v1beta1.Combine{
						Variables: []v1beta1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
							{FromFieldPath: "objectMeta.labels.source3"},
						},
						Strategy: v1beta1.CombineStrategyString,
						String:   &v1beta1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source3": "baz",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source3": "baz",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						}},
				},
				err: nil,
			},
		},
		"ValidCombineFromComposite": {
			reason: "Should correctly apply a CombineFromComposite patch with valid settings",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeCombineFromComposite,
					Combine: &v1beta1.Combine{
						Variables: []v1beta1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1beta1.CombineStrategyString,
						String:   &v1beta1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"Test":        "blah",
							"destination": "foo-bar",
						}},
				},
				err: nil,
			},
		},
		"ValidCombineToComposite": {
			reason: "Should correctly apply a CombineToComposite patch with valid settings",
			args: args{
				patch: v1beta1.Patch{
					Type: v1beta1.PatchTypeCombineToComposite,
					Combine: &v1beta1.Combine{
						Variables: []v1beta1.CombineVariable{
							{FromFieldPath: "objectMeta.labels.source1"},
							{FromFieldPath: "objectMeta.labels.source2"},
						},
						Strategy: v1beta1.CombineStrategyString,
						String:   &v1beta1.StringCombine{Format: "%s-%s"},
					},
					ToFieldPath: pointer.String("objectMeta.labels.destination"),
				},
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test": "blah",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						},
					},
				},
			},
			want: want{
				cp: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cp",
						Labels: map[string]string{
							"Test":        "blah",
							"destination": "foo-bar",
						},
					},
					ConnectionDetailsLastPublishedTimer: lpt,
				},
				cd: &fake.Composed{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cd",
						Labels: map[string]string{
							"source1": "foo",
							"source2": "bar",
						}},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ncp := tc.args.cp.DeepCopyObject().(resource.Composite)
			err := Apply(tc.args.patch, ncp, tc.args.cd, tc.args.only...)

			if tc.want.cp != nil {
				if diff := cmp.Diff(tc.want.cp, ncp); diff != "" {
					t.Errorf("\n%s\nApply(cp): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if tc.want.cd != nil {
				if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
					t.Errorf("\n%s\nApply(cd): -want, +got:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(err): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestOptionalFieldPathNotFound(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := func() error {
		p := &fieldpath.Paved{}
		_, err := p.GetValue("boom")
		return err
	}
	required := v1beta1.FromFieldPathPolicyRequired
	optional := v1beta1.FromFieldPathPolicyOptional
	type args struct {
		err error
		p   *v1beta1.PatchPolicy
	}

	cases := map[string]struct {
		reason string
		args
		want bool
	}{
		"NotAnError": {
			reason: "Should perform patch if no error finding field.",
			args:   args{},
			want:   false,
		},
		"NotFieldNotFoundError": {
			reason: "Should return error if something other than field not found.",
			args: args{
				err: errBoom,
			},
			want: false,
		},
		"DefaultOptionalNoPolicy": {
			reason: "Should return no-op if field not found and no patch policy specified.",
			args: args{
				err: errNotFound(),
			},
			want: true,
		},
		"DefaultOptionalNoPathPolicy": {
			reason: "Should return no-op if field not found and empty patch policy specified.",
			args: args{
				p:   &v1beta1.PatchPolicy{},
				err: errNotFound(),
			},
			want: true,
		},
		"OptionalNotFound": {
			reason: "Should return no-op if field not found and optional patch policy explicitly specified.",
			args: args{
				p: &v1beta1.PatchPolicy{
					FromFieldPath: &optional,
				},
				err: errNotFound(),
			},
			want: true,
		},
		"RequiredNotFound": {
			reason: "Should return error if field not found and required patch policy explicitly specified.",
			args: args{
				p: &v1beta1.PatchPolicy{
					FromFieldPath: &required,
				},
				err: errNotFound(),
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsOptionalFieldPathNotFound(tc.args.err, tc.args.p)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsOptionalFieldPathNotFound(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestComposedTemplates(t *testing.T) {
	asJSON := func(val interface{}) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		pss []v1beta1.PatchSet
		cts []v1beta1.ComposedTemplate
	}

	type want struct {
		ct  []v1beta1.ComposedTemplate
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoCompositionPatchSets": {
			reason: "Patches defined on a composite resource should be applied correctly if no PatchSets are defined on the composition",
			args: args{
				cts: []v1beta1.ComposedTemplate{
					{
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.name"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.namespace"),
							},
						},
					},
				},
			},
			want: want{
				ct: []v1beta1.ComposedTemplate{
					{
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.name"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.namespace"),
							},
						},
					},
				},
			},
		},
		"UndefinedPatchSet": {
			reason: "Should return error and not modify the patches field when referring to an undefined PatchSet",
			args: args{
				cts: []v1beta1.ComposedTemplate{{
					Patches: []v1beta1.Patch{
						{
							Type:         v1beta1.PatchTypePatchSet,
							PatchSetName: pointer.String("patch-set-1"),
						},
					},
				}},
			},
			want: want{
				err: errors.Errorf(errFmtUndefinedPatchSet, "patch-set-1"),
			},
		},
		"DefinedPatchSets": {
			reason: "Should de-reference PatchSets defined on the Composition when referenced in a composed resource",
			args: args{
				// PatchSets, existing patches and references
				// should output in the correct order.
				pss: []v1beta1.PatchSet{
					{
						Name: "patch-set-1",
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.namespace"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("spec.parameters.test"),
							},
						},
					},
					{
						Name: "patch-set-2",
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.annotations.patch-test-1"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.annotations.patch-test-2"),
								Transforms: []v1beta1.Transform{{
									Type: v1beta1.TransformTypeMap,
									Map: &v1beta1.MapTransform{
										Pairs: map[string]extv1.JSON{
											"k-1": asJSON("v-1"),
											"k-2": asJSON("v-2"),
										},
									},
								}},
							},
						},
					},
				},
				cts: []v1beta1.ComposedTemplate{
					{
						Patches: []v1beta1.Patch{
							{
								Type:         v1beta1.PatchTypePatchSet,
								PatchSetName: pointer.String("patch-set-2"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.name"),
							},
							{
								Type:         v1beta1.PatchTypePatchSet,
								PatchSetName: pointer.String("patch-set-1"),
							},
						},
					},
					{
						Patches: []v1beta1.Patch{
							{
								Type:         v1beta1.PatchTypePatchSet,
								PatchSetName: pointer.String("patch-set-1"),
							},
						},
					},
				},
			},
			want: want{
				err: nil,
				ct: []v1beta1.ComposedTemplate{
					{
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.annotations.patch-test-1"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.annotations.patch-test-2"),
								Transforms: []v1beta1.Transform{{
									Type: v1beta1.TransformTypeMap,
									Map: &v1beta1.MapTransform{
										Pairs: map[string]extv1.JSON{
											"k-1": asJSON("v-1"),
											"k-2": asJSON("v-2"),
										},
									},
								}},
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.name"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.namespace"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("spec.parameters.test"),
							},
						},
					},
					{
						Patches: []v1beta1.Patch{
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("metadata.namespace"),
							},
							{
								Type:          v1beta1.PatchTypeFromCompositeFieldPath,
								FromFieldPath: pointer.String("spec.parameters.test"),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ComposedTemplates(tc.args.pss, tc.args.cts)

			if diff := cmp.Diff(tc.want.ct, got); diff != "" {
				t.Errorf("\n%s\nrs.ComposedTemplates(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nrs.ComposedTemplates(...)): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolveTransforms(t *testing.T) {
	type args struct {
		ts    []v1beta1.Transform
		input any
	}
	type want struct {
		output any
		err    error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "NoTransforms",
			args: args{
				ts: nil,
				input: map[string]interface{}{
					"spec": map[string]interface{}{
						"parameters": map[string]interface{}{
							"test": "test",
						},
					},
				},
			},
			want: want{
				output: map[string]interface{}{
					"spec": map[string]interface{}{
						"parameters": map[string]interface{}{
							"test": "test",
						},
					},
				},
			},
		},
		{
			name: "MathTransformWithConversionToFloat64",
			args: args{
				ts: []v1beta1.Transform{{
					Type: v1beta1.TransformTypeConvert,
					Convert: &v1beta1.ConvertTransform{
						ToType: v1beta1.TransformIOTypeFloat64,
					},
				}, {
					Type: v1beta1.TransformTypeMath,
					Math: &v1beta1.MathTransform{
						Multiply: pointer.Int64(2),
					},
				}},
				input: int64(2),
			},
			want: want{
				output: float64(4),
			},
		},
		{
			name: "MathTransformWithConversionToInt64",
			args: args{
				ts: []v1beta1.Transform{{
					Type: v1beta1.TransformTypeConvert,
					Convert: &v1beta1.ConvertTransform{
						ToType: v1beta1.TransformIOTypeInt64,
					},
				}, {
					Type: v1beta1.TransformTypeMath,
					Math: &v1beta1.MathTransform{
						Multiply: pointer.Int64(2),
					},
				}},
				input: int64(2),
			},
			want: want{
				output: int64(4),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTransforms(v1beta1.Patch{Transforms: tt.args.ts}, tt.args.input)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveTransforms(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tt.want.output, got); diff != "" {
				t.Errorf("ResolveTransforms(...): -want, +got:\n%s", diff)
			}
		})
	}
}