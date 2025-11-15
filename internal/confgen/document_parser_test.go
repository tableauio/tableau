package confgen

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

func newDocParserForTest() *sheetParser {
	return NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
		book.MetabookOptions(),
		book.MetasheetOptions(context.Background()),
		&SheetParserExtInfo{
			InputDir:       "",
			SubdirRewrites: map[string]string{},
			BookFormat:     format.YAML,
		})
}

func TestTableParser_parseDocumentMetasheet(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		wantMsg proto.Message
	}{
		{
			name:    "parse yaml metasheet",
			parser:  newDocParserForTest(),
			args:    args{path: "./testdata/Metasheet.yaml"},
			wantErr: false,
			wantMsg: &internalpb.Metabook{
				MetasheetMap: map[string]*internalpb.Metasheet{
					"HeroConf": {
						Sheet: "HeroConf",
					},
					"ItemConf": {
						Sheet:      "ItemConf",
						Alias:      "ItemAliasConf",
						OrderedMap: true,
					},
				},
			},
		},
		{
			name:    "parse xml metasheet",
			parser:  newDocParserForTest(),
			args:    args{path: "./testdata/Metasheet.xml"},
			wantErr: false,
			wantMsg: &internalpb.Metabook{
				MetasheetMap: map[string]*internalpb.Metasheet{
					"ItemConf": {
						Sheet:      "ItemConf",
						OrderedMap: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp, err := importer.New(context.Background(), tt.args.path, importer.Parser(tt.parser))
			if err != nil {
				t.Fatal(err)
			}
			sheet := imp.GetSheet(metasheet.DefaultMetasheetName)
			if sheet == nil {
				t.Fatalf("metasheet not found")
			}
			msg := &internalpb.Metabook{}
			err = tt.parser.Parse(msg, sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %s, wantErr %v", xerrors.NewDesc(err), tt.wantErr)
			}
			fmt.Println("sheet:", sheet)
			fmt.Println("metabook:", msg)
			if !proto.Equal(msg, tt.wantMsg) {
				t.Errorf("\ngotMsg: %v\nwantMsg: %v", msg, tt.wantMsg)
			}
		})
	}
}

func TestDocParser_parseFieldNotFound(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		err     error
	}{
		{
			name:   "no duplicate key",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "YamlScalarConf",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "YamlScalarConf",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Name: "",
								Children: []*book.Node{
									{
										Name:  "ID",
										Value: "1",
									},
									{
										Name:  "Num",
										Value: "10",
									},
									{
										Name:  "Value",
										Value: "20",
									},
									{
										Name:  "Weight",
										Value: "30",
									},
									{
										Name:  "Percentage",
										Value: "0.5",
									},
									{
										Name:  "Ratio",
										Value: "1.5",
									},
									{
										Name:  "Name",
										Value: "Apple",
									},
									{
										Name:  "Blob",
										Value: "VGFibGVhdQ==",
									},
									{
										Name:  "OK",
										Value: "true",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "field-not-found",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "YamlScalarConf",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "YamlScalarConf",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Name: "",
								Children: []*book.Node{
									{
										Name:  "ID",
										Value: "1",
									},
									{
										Name:  "Num",
										Value: "10",
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2014,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.YamlScalarConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestDocParser_parseDocumentUniqueFieldStructList(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		err     error
	}{
		{
			name:   "no duplicate key",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructList",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructList",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.ListNode,
										Name: "Items",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001",
													},
													{
														Name:  "Name",
														Value: "Apple",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1002",
													},
													{
														Name:  "Name",
														Value: "Banana",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1003",
													},
													{
														Name:  "Name",
														Value: "Orange",
													},
													{
														Name:  "Num",
														Value: "20",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate id",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructList",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructList",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.ListNode,
										Name: "Items",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001",
													},
													{
														Name:  "Name",
														Value: "Apple",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001", // duplicate
													},
													{
														Name:  "Name",
														Value: "Banana",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1003",
													},
													{
														Name:  "Name",
														Value: "Orange",
													},
													{
														Name:  "Num",
														Value: "20",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2022,
		},
		{
			name:   "duplicate name",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructList",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructList",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.ListNode,
										Name: "Items",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001",
													},
													{
														Name:  "Name",
														Value: "Apple",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1002",
													},
													{
														Name:  "Name",
														Value: "Banana",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1003",
													},
													{
														Name:  "Name",
														Value: "Banana", // duplicate
													},
													{
														Name:  "Num",
														Value: "20",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2022,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.DocumentUniqueFieldStructList{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %+v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				t.Logf("err: %+v", err)
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestDocParser_parseDocumentUniqueFieldStructMap(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		err     error
	}{
		{
			name:   "no duplicate key",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "Chapter",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "1001",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "3",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionThree",
																	},
																},
															},
														},
													},
												},
											},
											{
												Kind: book.MapNode,
												Name: "1002",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterTwo",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate chapter name",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "Chapter",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "1001",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "3",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionThree",
																	},
																},
															},
														},
													},
												},
											},
											{
												Kind: book.MapNode,
												Name: "1002",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne", // duplicate
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2022,
		},
		{
			name:   "duplicate section name",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "Chapter",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "1001",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "3",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionThree",
																	},
																},
															},
														},
													},
												},
											},
											{
												Kind: book.MapNode,
												Name: "1002",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterTwo",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne", // duplicate
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2022,
		},
		{
			name:   "duplicate chapter id",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "Chapter",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "1001",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "3",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionThree",
																	},
																},
															},
														},
													},
												},
											},
											{
												Kind: book.MapNode,
												Name: "1001", // duplicate
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterTwo",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
															{
																Kind: book.MapNode,
																Name: "2",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionTwo",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2005,
		},
		{
			name:   "duplicate scalar map key",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "ScalarMap",
										Children: []*book.Node{
											{
												Name:  "1",
												Value: "dog",
											},
											{
												Name:  "2",
												Value: "bird",
											},
											{
												Name:  "2",
												Value: "cat",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2005,
		},
		{
			name:   "duplicate incell map key",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Name:  "IncellMap",
										Value: "1:dog,2:bird,2:cat",
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2005,
		},
		{
			name:   "card prefix uniqueness",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentUniqueFieldStructMap",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentUniqueFieldStructMap",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.MapNode,
										Name: "Chapter",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "_infox",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "1",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																},
															},
														},
													},
												},
											},
										},
									},
									{
										Kind: book.MapNode,
										Name: "ChapterInfo",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Name: "x",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterOne",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "section",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																	{
																		Kind: book.MapNode,
																		Name: "Section",
																		Children: []*book.Node{
																			{
																				Kind: book.MapNode,
																				Name: "section",
																				Children: []*book.Node{
																					{
																						Name:  "Name",
																						Value: "SectionOne",
																					},
																					{
																						Kind: book.MapNode,
																						Name: "Section",
																						Children: []*book.Node{
																							{
																								Kind: book.MapNode,
																								Name: "section",
																								Children: []*book.Node{
																									{
																										Name:  "Name",
																										Value: "SectionOne",
																									},
																									{
																										Kind: book.MapNode,
																										Name: "Section",
																										Children: []*book.Node{
																											{
																												Kind: book.MapNode,
																												Name: "section",
																												Children: []*book.Node{
																													{
																														Name:  "Name",
																														Value: "SectionOne",
																													},
																												},
																											},
																										},
																									},
																								},
																							},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
											{
												Kind: book.MapNode,
												Name: "x.section",
												Children: []*book.Node{
													{
														Name:  "Name",
														Value: "ChapterTwo",
													},
													{
														Kind: book.MapNode,
														Name: "Section",
														Children: []*book.Node{
															{
																Kind: book.MapNode,
																Name: "section.section",
																Children: []*book.Node{
																	{
																		Name:  "Name",
																		Value: "SectionOne",
																	},
																	{
																		Kind: book.MapNode,
																		Name: "Section",
																		Children: []*book.Node{
																			{
																				Kind: book.MapNode,
																				Name: "section",
																				Children: []*book.Node{
																					{
																						Name:  "Name",
																						Value: "SectionOne",
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.DocumentUniqueFieldStructMap{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %+v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestDocParser_parseSequenceUniqueFieldStructList(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		err     error
	}{
		{
			name:   "in sequence",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentSequenceFieldStructList",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentSequenceFieldStructList",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.ListNode,
										Name: "Items",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001",
													},
													{
														Name:  "Name",
														Value: "Apple",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1002",
													},
													{
														Name:  "Name",
														Value: "Banana",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1003",
													},
													{
														Name:  "Name",
														Value: "Orange",
													},
													{
														Name:  "Num",
														Value: "20",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "id not in sequence",
			parser: newDocParserForTest(),
			args: args{
				sheet: &book.Sheet{
					Name: "DocumentSequenceFieldStructList",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "DocumentSequenceFieldStructList",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Children: []*book.Node{
									{
										Kind: book.ListNode,
										Name: "Items",
										Children: []*book.Node{
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1001",
													},
													{
														Name:  "Name",
														Value: "Apple",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1003", // not in sequence
													},
													{
														Name:  "Name",
														Value: "Banana",
													},
													{
														Name:  "Num",
														Value: "10",
													},
												},
											},
											{
												Kind: book.MapNode,
												Children: []*book.Node{
													{
														Name:  "ID",
														Value: "1004",
													},
													{
														Name:  "Name",
														Value: "Orange",
													},
													{
														Name:  "Num",
														Value: "20",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			err:     xerrors.ErrE2003,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.DocumentSequenceFieldStructList{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %+v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}
