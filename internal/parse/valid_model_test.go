package parse_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// The spec cases only check findings; these tests check the model the rules
// work on. Every expected model is written by hand from the YAML file, with
// the position of every value and key.

func TestMinimalModel(t *testing.T) {
	checkModel(t, "valid/01-minimal.yml", &model.Pipeline{
		Resources: []*model.Resource{{
			Position: at(3, 3),
			Name:     stringField("repo", 3, 3, 9),
			Type:     stringField("git", 4, 3, 9),
			Source: mappingNode(6, 5,
				pair("uri", 6, 5, scalar("https://github.com/example/app.git", 6, 10)),
			),
		}},
		Jobs: []*model.Job{{
			Position: at(9, 3),
			Name:     stringField("unit", 9, 3, 9),
			Plan: []*model.Step{
				{Position: at(11, 5), Get: &model.GetStep{
					Name:    stringField("repo", 11, 5, 10),
					Trigger: boolField(true, 12, 5, 14),
				}},
				{Position: at(13, 5), Task: &model.TaskStep{
					Name: stringField("test", 13, 5, 11),
					File: stringField("repo/ci/test.yml", 14, 5, 11),
				}},
			},
		}},
	})
}

func TestHooksAndNestingModel(t *testing.T) {
	checkModel(t, "valid/05-hooks-and-nesting.yml", &model.Pipeline{
		Resources: []*model.Resource{
			{
				Position: at(4, 3),
				Name:     stringField("repo", 4, 3, 9),
				Type:     stringField("git", 5, 3, 9),
				Source: mappingNode(6, 11,
					pair("uri", 6, 12, scalar("https://github.com/example/app.git", 6, 17)),
				),
			},
			{
				Position: at(7, 3),
				Name:     stringField("artifacts", 7, 3, 9),
				Type:     stringField("s3", 8, 3, 9),
				Source: mappingNode(10, 5,
					pair("bucket", 10, 5, scalar("build-artifacts", 10, 13)),
					pair("regexp", 11, 5, scalar("app-(.*).tgz", 11, 13)),
				),
			},
		},
		Jobs: []*model.Job{
			{
				Position: at(14, 3),
				Name:     stringField("reconfigure", 14, 3, 9),
				Plan: []*model.Step{
					{Position: at(16, 5), Get: &model.GetStep{
						Name:    stringField("repo", 16, 5, 10),
						Trigger: boolField(true, 17, 5, 14),
					}},
					{Position: at(18, 5), SetPipeline: &model.SetPipelineStep{
						Name: stringField("self", 18, 5, 19),
						File: stringField("repo/ci/pipeline.yml", 19, 5, 11),
					}},
				},
			},
			{
				Position: at(21, 3),
				Name:     stringField("package", 21, 3, 9),
				Plan: []*model.Step{
					{Position: at(23, 5), Get: &model.GetStep{
						Name:    stringField("repo", 23, 5, 10),
						Trigger: boolField(true, 24, 5, 14),
						Passed:  []model.Value[string]{listItem("reconfigure", 25, 14)},
					}},
					{Position: at(26, 5), Task: &model.TaskStep{
						Name: stringField("generate-tasks", 26, 5, 11),
						Config: &model.TaskConfig{
							Position: at(28, 7),
							Platform: stringField("linux", 28, 7, 17),
							ImageResource: &model.ImageResource{
								Position: at(30, 9),
								Type:     stringField("registry-image", 30, 9, 15),
								Source: mappingNode(31, 17,
									pair("repository", 31, 18, scalar("alpine", 31, 30)),
									pair("tag", 31, 38, quoted("3.20", 31, 43)),
								),
							},
							Inputs: []*model.TaskInput{{
								Position: at(32, 17),
								Name:     stringField("repo", 32, 17, 23),
							}},
							Outputs: []*model.TaskOutput{{
								Position: at(33, 18),
								Name:     stringField("generated", 33, 18, 24),
							}},
							Run: &model.TaskRun{
								Position: at(34, 13),
								Path:     stringField("repo/ci/generate.sh", 34, 13, 19),
							},
						},
					}},
					{Position: at(35, 5), Task: &model.TaskStep{
						Name: stringField("run-generated", 35, 5, 11),
						File: stringField("generated/task.yml", 36, 5, 11),
					}},
					{Position: at(37, 5), Do: &model.DoStep{Steps: []*model.Step{
						{Position: at(38, 7), Try: &model.TryStep{Step: &model.Step{
							Position: at(39, 9),
							Put: &model.PutStep{
								Name: stringField("artifacts", 39, 9, 14),
								Params: mappingNode(40, 17,
									pair("file", 40, 18, scalar("generated/app-*.tgz", 40, 24)),
								),
							},
						}}},
						{
							Position: at(41, 7),
							Task: &model.TaskStep{
								Name: stringField("verify", 41, 7, 13),
								File: stringField("repo/ci/verify.yml", 42, 7, 13),
							},
							Hooks: model.Hooks{OnSuccess: &model.Step{
								Position: at(44, 9),
								Do: &model.DoStep{Steps: []*model.Step{
									{Position: at(45, 11), Task: &model.TaskStep{
										Name: stringField("report", 45, 11, 17),
										File: stringField("repo/ci/report.yml", 46, 11, 17),
									}},
								}},
							}},
						},
					}}},
				},
			},
		},
	})
}

func TestFullFeaturedModel(t *testing.T) {
	checkModel(t, "valid/02-full-featured.yml", &model.Pipeline{
		ResourceTypes: []*model.ResourceType{{
			Position: at(3, 3),
			Name:     stringField("slack-notification", 3, 3, 9),
			Type:     stringField("registry-image", 4, 3, 9),
			Source: mappingNode(6, 5,
				pair("repository", 6, 5, scalar("cfcommunity/slack-notification-resource", 6, 17)),
				pair("tag", 7, 5, quoted("1.7.0", 7, 10)),
			),
		}},
		Resources: []*model.Resource{
			{
				Position: at(10, 3),
				Name:     stringField("repo", 10, 3, 9),
				Type:     stringField("git", 11, 3, 9),
				Icon:     stringField("github", 12, 3, 9),
				Source: mappingNode(14, 5,
					pair("uri", 14, 5, scalar("((git-uri))", 14, 10)),
					pair("branch", 15, 5, scalar("main", 15, 13)),
					pair("private_key", 16, 5, scalar("((git-private-key))", 16, 18)),
				),
			},
			{
				Position: at(18, 3),
				Name:     stringField("version", 18, 3, 9),
				Type:     stringField("semver", 19, 3, 9),
				Source: mappingNode(21, 5,
					pair("driver", 21, 5, scalar("git", 21, 13)),
					pair("uri", 22, 5, scalar("((git-uri))", 22, 10)),
					pair("branch", 23, 5, scalar("version", 23, 13)),
					pair("file", 24, 5, scalar("version", 24, 11)),
				),
			},
			{
				Position: at(26, 3),
				Name:     stringField("image", 26, 3, 9),
				Type:     stringField("registry-image", 27, 3, 9),
				Source: mappingNode(29, 5,
					pair("repository", 29, 5, scalar("registry.example.com/app", 29, 17)),
					pair("username", 30, 5, scalar("((registry.user))", 30, 15)),
					pair("password", 31, 5, scalar("((registry.password))", 31, 15)),
				),
			},
			{
				Position: at(33, 3),
				Name:     stringField("nightly", 33, 3, 9),
				Type:     stringField("time", 34, 3, 9),
				Source: mappingNode(36, 5,
					pair("interval", 36, 5, scalar("24h", 36, 15)),
				),
			},
			{
				Position: at(38, 3),
				Name:     stringField("notify", 38, 3, 9),
				Type:     stringField("slack-notification", 39, 3, 9),
				Source: mappingNode(41, 5,
					pair("url", 41, 5, scalar("((slack-webhook))", 41, 10)),
				),
			},
		},
		Groups: []*model.Group{
			{
				Position: at(44, 3),
				Name:     stringField("build", 44, 3, 9),
				Jobs:     []model.Value[string]{listItem("test", 45, 10), listItem("build-image", 45, 16)},
			},
			{
				Position: at(46, 3),
				Name:     stringField("deploy", 46, 3, 9),
				Jobs:     []model.Value[string]{listItem("deploy", 47, 10)},
			},
		},
		Jobs: []*model.Job{
			{
				Position: at(50, 3),
				Name:     stringField("test", 50, 3, 9),
				Plan: []*model.Step{
					{Position: at(52, 5), InParallel: &model.InParallelStep{Steps: []*model.Step{
						{Position: at(53, 7), Get: &model.GetStep{
							Name:    stringField("repo", 53, 7, 12),
							Trigger: boolField(true, 54, 7, 16),
						}},
						{Position: at(55, 7), Get: &model.GetStep{
							Name:    stringField("nightly", 55, 7, 12),
							Trigger: boolField(true, 56, 7, 16),
						}},
					}}},
					{Position: at(57, 5), Task: &model.TaskStep{
						Name: stringField("unit", 57, 5, 11),
						File: stringField("repo/ci/tasks/unit.yml", 58, 5, 11),
					}},
				},
				Hooks: model.Hooks{OnFailure: &model.Step{
					Position: at(60, 5),
					Put: &model.PutStep{
						Name: stringField("notify", 60, 5, 10),
						Params: mappingNode(62, 7,
							pair("text", 62, 7, quoted("Tests failed", 62, 13)),
						),
					},
				}},
			},
			{
				Position: at(64, 3),
				Name:     stringField("build-image", 64, 3, 9),
				Serial:   boolField(true, 65, 3, 11),
				Plan: []*model.Step{
					{Position: at(67, 5), InParallel: &model.InParallelStep{
						FailFast: boolField(true, 68, 7, 18),
						Limit:    intField(2, 69, 7, 14),
						Steps: []*model.Step{
							{Position: at(71, 9), Get: &model.GetStep{
								Name:    stringField("repo", 71, 9, 14),
								Passed:  []model.Value[string]{listItem("test", 72, 18)},
								Trigger: boolField(true, 73, 9, 18),
							}},
							{Position: at(74, 9), Get: &model.GetStep{
								Name: stringField("version", 74, 9, 14),
								Params: mappingNode(75, 17,
									pair("bump", 75, 18, scalar("patch", 75, 24)),
								),
							}},
						},
					}},
					{Position: at(76, 5), Task: &model.TaskStep{
						Name:       stringField("build", 76, 5, 11),
						Privileged: boolField(true, 77, 5, 17),
						Config: &model.TaskConfig{
							Position: at(79, 7),
							Platform: stringField("linux", 79, 7, 17),
							ImageResource: &model.ImageResource{
								Position: at(81, 9),
								Type:     stringField("registry-image", 81, 9, 15),
								Source: mappingNode(82, 17,
									pair("repository", 82, 18, scalar("concourse/oci-build-task", 82, 30)),
									pair("tag", 82, 56, quoted("0.11.1", 82, 61)),
								),
							},
							Inputs: []*model.TaskInput{{
								Position: at(84, 9),
								Name:     stringField("repo", 84, 9, 15),
							}},
							Outputs: []*model.TaskOutput{{
								Position: at(86, 9),
								Name:     stringField("image", 86, 9, 15),
							}},
							Params: mappingNode(88, 9,
								pair("CONTEXT", 88, 9, scalar("repo", 88, 18)),
							),
							Run: &model.TaskRun{
								Position: at(90, 9),
								Path:     stringField("build", 90, 9, 15),
							},
						},
					}},
					{Position: at(91, 5), Put: &model.PutStep{
						Name: stringField("image", 91, 5, 10),
						Params: mappingNode(93, 7,
							pair("image", 93, 7, scalar("image/image.tar", 93, 14)),
							pair("version", 94, 7, scalar("version/version", 94, 16)),
						),
					}},
					{Position: at(95, 5), Put: &model.PutStep{
						Name: stringField("version", 95, 5, 10),
						Params: mappingNode(96, 13,
							pair("file", 96, 14, scalar("version/version", 96, 20)),
						),
					}},
				},
			},
			{
				Position: at(98, 3),
				Name:     stringField("deploy", 98, 3, 9),
				Plan: []*model.Step{
					{Position: at(100, 5), InParallel: &model.InParallelStep{Steps: []*model.Step{
						{Position: at(101, 7), Get: &model.GetStep{
							Name:    stringField("image", 101, 7, 12),
							Passed:  []model.Value[string]{listItem("build-image", 102, 16)},
							Trigger: boolField(true, 103, 7, 16),
							Params: mappingNode(104, 15,
								pair("skip_download", 104, 16, scalar("true", 104, 31)),
							),
						}},
						{Position: at(105, 7), Get: &model.GetStep{
							Name:   stringField("version", 105, 7, 12),
							Passed: []model.Value[string]{listItem("build-image", 106, 16)},
						}},
						{Position: at(107, 7), Get: &model.GetStep{
							Name:   stringField("repo", 107, 7, 12),
							Passed: []model.Value[string]{listItem("build-image", 108, 16)},
						}},
					}}},
					{Position: at(109, 5), LoadVar: &model.LoadVarStep{
						Name: stringField("image-version", 109, 5, 15),
						File: stringField("version/version", 110, 5, 11),
					}},
					{
						Position: at(111, 5),
						Task: &model.TaskStep{
							Name: stringField("deploy", 111, 5, 11),
							File: stringField("repo/ci/tasks/deploy.yml", 112, 5, 11),
							Params: mappingNode(118, 7,
								pair("ENVIRONMENT", 118, 7, scalar("((.:env))", 118, 20)),
								pair("VERSION", 119, 7, scalar("((.:image-version))", 119, 16)),
							),
						},
						Across: &model.Across{Variables: []*model.AcrossVariable{{
							Position: at(114, 7),
							Name:     stringField("env", 114, 7, 12),
							Values: listNode(115, 15,
								scalar("staging", 115, 16),
								scalar("production", 115, 25),
							),
							MaxInFlight: scalar("1", 116, 22),
						}}},
					},
				},
				Hooks: model.Hooks{Ensure: &model.Step{
					Position: at(121, 5),
					Put: &model.PutStep{
						Name: stringField("notify", 121, 5, 10),
						Params: mappingNode(123, 7,
							pair("text", 123, 7, quoted("Deploy finished", 123, 13)),
						),
					},
				}},
			},
		},
	})
}

func TestGetAliasModel(t *testing.T) {
	checkModel(t, "valid/04-get-alias.yml", &model.Pipeline{
		Resources: []*model.Resource{{
			Position: at(4, 3),
			Name:     stringField("app-repo", 4, 3, 9),
			Type:     stringField("git", 5, 3, 9),
			Source: mappingNode(6, 11,
				pair("uri", 6, 12, scalar("https://github.com/example/app.git", 6, 17)),
			),
		}},
		Jobs: []*model.Job{
			{
				Position: at(9, 3),
				Name:     stringField("test", 9, 3, 9),
				Plan: []*model.Step{
					{Position: at(11, 5), Get: &model.GetStep{
						Name:     stringField("source", 11, 5, 10),
						Resource: stringField("app-repo", 12, 5, 15),
						Trigger:  boolField(true, 13, 5, 14),
					}},
					{Position: at(14, 5), Task: &model.TaskStep{
						Name: stringField("test", 14, 5, 11),
						File: stringField("source/ci/test.yml", 15, 5, 11),
					}},
				},
			},
			{
				Position: at(16, 3),
				Name:     stringField("release", 16, 3, 9),
				Plan: []*model.Step{
					{Position: at(18, 5), Get: &model.GetStep{
						Name:     stringField("source", 18, 5, 10),
						Resource: stringField("app-repo", 19, 5, 15),
						Passed:   []model.Value[string]{listItem("test", 20, 14)},
					}},
					{Position: at(21, 5), Put: &model.PutStep{
						Name:     stringField("tag-release", 21, 5, 10),
						Resource: stringField("app-repo", 22, 5, 15),
						Params: mappingNode(23, 13,
							pair("repository", 23, 14, scalar("source", 23, 26)),
							pair("tag", 23, 34, scalar("source/VERSION", 23, 39)),
						),
					}},
				},
			},
		},
	})
}

func TestVarsModel(t *testing.T) {
	checkModel(t, "valid/06-vars.yml", &model.Pipeline{
		Resources: []*model.Resource{{
			Position: at(5, 3),
			Name:     stringField("repo", 5, 3, 9),
			Type:     stringField("git", 6, 3, 9),
			Source: mappingNode(8, 5,
				pair("uri", 8, 5, scalar("((repo.uri))", 8, 10)),
				pair("branch", 9, 5, scalar("((branch))", 9, 13)),
				pair("private_key", 10, 5, scalar("((repo.private_key))", 10, 18)),
			),
		}},
		Jobs: []*model.Job{{
			Position: at(13, 3),
			Name:     stringField("build", 13, 3, 9),
			Plan: []*model.Step{
				{Position: at(15, 5), Get: &model.GetStep{
					Name:    stringField("repo", 15, 5, 10),
					Trigger: boolField(true, 16, 5, 14),
				}},
				{Position: at(17, 5), LoadVar: &model.LoadVarStep{
					Name: stringField("build-number", 17, 5, 15),
					File: stringField("repo/.build-number", 18, 5, 11),
				}},
				{Position: at(19, 5), InParallel: &model.InParallelStep{Steps: []*model.Step{
					{Position: at(20, 7), Task: &model.TaskStep{
						Name: stringField("build-linux", 20, 7, 13),
						File: stringField("repo/ci/build.yml", 21, 7, 13),
						Vars: mappingNode(23, 9,
							pair("os", 23, 9, scalar("linux", 23, 13)),
							pair("number", 24, 9, scalar("((.:build-number))", 24, 17)),
						),
					}},
					{Position: at(25, 7), Task: &model.TaskStep{
						Name: stringField("build-darwin", 25, 7, 13),
						File: stringField("repo/ci/((build-task)).yml", 26, 7, 13),
						Vars: mappingNode(28, 9,
							pair("os", 28, 9, scalar("darwin", 28, 13)),
							pair("number", 29, 9, scalar("((.:build-number))", 29, 17)),
						),
					}},
				}}},
			},
		}},
	})
}

func TestBuiltinTypesModel(t *testing.T) {
	checkModel(t, "valid/07-builtin-types.yml", &model.Pipeline{
		Resources: []*model.Resource{
			{
				Position: at(4, 4),
				Name:     stringField("src", 4, 4, 10),
				Type:     stringField("git", 4, 15, 21),
				Source: mappingNode(4, 34,
					pair("uri", 4, 35, quoted("https://github.com/example/app.git", 4, 40)),
				),
			},
			{
				Position: at(5, 4),
				Name:     stringField("every-hour", 5, 4, 10),
				Type:     stringField("time", 5, 22, 28),
				Source: mappingNode(5, 42,
					pair("interval", 5, 43, scalar("1h", 5, 53)),
				),
			},
			{
				Position: at(6, 4),
				Name:     stringField("base-image", 6, 4, 10),
				Type:     stringField("registry-image", 6, 22, 28),
				Source: mappingNode(6, 52,
					pair("repository", 6, 53, scalar("alpine", 6, 65)),
				),
			},
			{
				Position: at(7, 4),
				Name:     stringField("bucket", 7, 4, 10),
				Type:     stringField("s3", 7, 18, 24),
				Source: mappingNode(7, 36,
					pair("bucket", 7, 37, scalar("b", 7, 45)),
					pair("regexp", 7, 48, quoted("x-(.*).tgz", 7, 56)),
				),
			},
			{
				Position: at(8, 4),
				Name:     stringField("ver", 8, 4, 10),
				Type:     stringField("semver", 8, 15, 21),
				Source: mappingNode(8, 37,
					pair("driver", 8, 38, scalar("s3", 8, 46)),
					pair("bucket", 8, 50, scalar("b", 8, 58)),
					pair("key", 8, 61, scalar("version", 8, 66)),
				),
			},
			{
				Position: at(9, 4),
				Name:     stringField("lock", 9, 4, 10),
				Type:     stringField("pool", 9, 16, 22),
				Source: mappingNode(9, 36,
					pair("uri", 9, 37, quoted("git@example.com:locks.git", 9, 42)),
					pair("branch", 9, 71, scalar("main", 9, 79)),
					pair("pool", 9, 85, scalar("env", 9, 91)),
				),
			},
			{
				Position: at(10, 4),
				Name:     stringField("release", 10, 4, 10),
				Type:     stringField("github-release", 10, 19, 25),
				Source: mappingNode(10, 49,
					pair("owner", 10, 50, scalar("example", 10, 57)),
					pair("repository", 10, 66, scalar("app", 10, 78)),
				),
			},
			{
				Position: at(11, 4),
				Name:     stringField("fake", 11, 4, 10),
				Type:     stringField("mock", 11, 16, 22),
				Source:   mappingNode(11, 36),
			},
		},
		Jobs: []*model.Job{{
			Position: at(14, 3),
			Name:     stringField("all", 14, 3, 9),
			Plan: []*model.Step{
				{Position: at(16, 5), InParallel: &model.InParallelStep{Steps: []*model.Step{
					{Position: at(17, 7), Get: &model.GetStep{Name: stringField("src", 17, 7, 12)}},
					{Position: at(18, 7), Get: &model.GetStep{
						Name:    stringField("every-hour", 18, 7, 12),
						Trigger: boolField(true, 19, 7, 16),
					}},
					{Position: at(20, 7), Get: &model.GetStep{Name: stringField("base-image", 20, 7, 12)}},
					{Position: at(21, 7), Get: &model.GetStep{Name: stringField("bucket", 21, 7, 12)}},
					{Position: at(22, 7), Get: &model.GetStep{Name: stringField("ver", 22, 7, 12)}},
					{Position: at(23, 7), Get: &model.GetStep{Name: stringField("release", 23, 7, 12)}},
					{Position: at(24, 7), Get: &model.GetStep{Name: stringField("fake", 24, 7, 12)}},
				}}},
				{Position: at(25, 5), Put: &model.PutStep{
					Name: stringField("lock", 25, 5, 10),
					Params: mappingNode(26, 13,
						pair("acquire", 26, 14, scalar("true", 26, 23)),
					),
				}},
			},
		}},
	})
}

// TestAnchorsModel: aliased content is decoded again at every use and keeps
// the positions of its definition.
func TestAnchorsModel(t *testing.T) {
	gitSource := []*model.NodePair{
		pair("uri", 6, 5, scalar("https://github.com/example/app.git", 6, 10)),
		pair("branch", 7, 5, scalar("main", 7, 13)),
	}
	runCheck := func() *model.Step {
		return &model.Step{Position: at(20, 5), Task: &model.TaskStep{
			Name: stringField("check", 20, 5, 11),
			File: stringField("repo/ci/check.yml", 21, 5, 11),
		}}
	}
	checkModel(t, "valid/03-anchors.yml", &model.Pipeline{
		Resources: []*model.Resource{
			{
				Position: at(3, 3),
				Name:     stringField("repo", 3, 3, 9),
				Type:     stringField("git", 4, 3, 9),
				Source:   mappingNode(6, 5, gitSource...),
			},
			{
				Position: at(8, 3),
				Name:     stringField("repo-docs", 8, 3, 9),
				Type:     stringField("git", 9, 3, 9),
				Source: mappingNode(11, 5, append(gitSource,
					pair("paths", 12, 5, listNode(12, 12, scalar("docs/", 12, 13))),
				)...),
			},
		},
		Jobs: []*model.Job{
			{
				Position: at(15, 3),
				Name:     stringField("lint", 15, 3, 9),
				Plan: []*model.Step{
					{Position: at(17, 5), Get: &model.GetStep{
						Name:    stringField("repo", 17, 5, 10),
						Trigger: boolField(true, 18, 5, 14),
					}},
					runCheck(),
				},
			},
			{
				Position: at(22, 3),
				Name:     stringField("docs", 22, 3, 9),
				Plan: []*model.Step{
					{Position: at(24, 5), Get: &model.GetStep{Name: stringField("repo", 24, 5, 10)}},
					{Position: at(25, 5), Get: &model.GetStep{
						Name:    stringField("repo-docs", 25, 5, 10),
						Trigger: boolField(true, 26, 5, 14),
					}},
					runCheck(),
				},
			},
		},
	})
}

// TestYAML11BooleansModel: yes and on are true, like in Concourse.
func TestYAML11BooleansModel(t *testing.T) {
	checkModel(t, "valid/08-yaml11-booleans.yml", &model.Pipeline{
		Resources: []*model.Resource{{
			Position: at(5, 3),
			Name:     stringField("repo", 5, 3, 9),
			Type:     stringField("git", 6, 3, 9),
			Source: mappingNode(7, 11,
				pair("uri", 7, 12, scalar("https://github.com/example/app.git", 7, 17)),
			),
		}},
		Jobs: []*model.Job{{
			Position:      at(10, 3),
			Name:          stringField("test", 10, 3, 9),
			Serial:        boolField(true, 11, 3, 11),
			Interruptible: boolField(true, 12, 3, 18),
			Plan: []*model.Step{
				{Position: at(14, 5), Get: &model.GetStep{
					Name:    stringField("repo", 14, 5, 10),
					Trigger: boolField(true, 15, 5, 14),
				}},
			},
		}},
	})
}
