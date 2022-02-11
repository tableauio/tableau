# Metadata Conversion

## Notation
The syntax is specified using [Extended Backus-Naur Form (EBNF)](https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form).

## Workbook -> Protoconf

### Basic

workbook: `(AliasTest)DemoTest`, worksheet: `(AliasActivity)DemoActivity`

- protoconf file name is `alias_test.proto`. If with no `()`, name will be `demo_test.proto`
- configuration message name is `AliasActivity`. If with no `()`, name will be `DemoActivity`
- list: `[ELEM-TYPE]COLUMN-TYPE`,  COLUMN-TYPE is column type, ELEM-TYPE is message name and list prefix (must not conflict with the protobuf keyword).
- map: `map<KEY-TYPE,VALUE-TYPE>`, KEY-TYPE must be scalar types, and VALUE-TYPE is message name and map prefix (must not conflict with build-in scalar type).
- import message types: `.TYPE`, e.g.: `.Item` represents the message `Item` already defined in the same protobuf package, and should not redefine it.
- well-known types
  - Timestamp: `google.protobuf.Timestamp`
  - Duration: `google.protobuf.Duration`

| ActivityID           | ActivityName | ActivityBeginTime   | ActivityDuration | ChapterID           | ChapterName | SectionID       | SectionName | SectionItem1Id | SectionItem1Num | SectionItem2Id | SectionItem2Num |
| -------------------- | ------------ | ------------------- | ---------------- | ------------------- | ----------- | --------------- | ----------- | -------------- | --------------- | -------------- | --------------- |
| map<uint32,Activity> | string       | timestamp           | duration         | map<uint32,Chapter> | string      | [Section]uint32 | int32       | [.Item]int32   | int32           | int32          | int32           |
| 1                    | activity1    | 2020-01-01 05:00:00 | 72h              | 1                   | chapter1    | 1               | section1    | 1001           | 1               | 1002           | 2               |
| 1                    | activity1    | 2020-01-01 05:00:00 | 72h              | 1                   | chapter1    | 2               | section2    | 1001           | 1               | 1002           | 2               |
| 1                    | activity1    | 2020-01-01 05:00:00 | 72h              | 2                   | chapter2    | 1               | section1    | 1001           | 1               | 1002           | 2               |
| 2                    | activity2    | 2020-01-01 05:00:00 | 72h3m0.5s        | 1                   | chapter1    | 1               | section1    | 1001           | 1               | 1002           | 2               |

```
// common.proto
message Item {
	int32 id = 1 [(tableau.field).name = "Id"];
	int32 num= 2 [(tableau.field).name = "Num"];
}
```

#### Output without prefix
```
// demo_test.proto
import "common.proto"

message DemoActivity{
	map<uint32, Activity> activity_map = 1 [(key) = "ActivityID"];
	message Activity {
		uint32 id= 1 [(tableau.field).name = "ActivityID"];
		string name = 2 [(tableau.field).name = "ActivityName"];
		map<uint32, Chapter> chapter_map = 3 [(tableau.field).key = "ChapterID"];
	}
	message Chapter {
		uint32 id= 1 [(tableau.field).name = "ChapterID"];
		string name = 2 [(tableau.field).name = "ChapterName"];
		repeated Section section_list = 3 [(tableau.field).layout = LAYOUT_VERTICAL];
	}
	message Section {
		uint32 id= 1 [(tableau.field).name = "SectionID"];
		string name = 2 [(tableau.field).name = "SectionName"];
		repeated Item item_list = 3 [(tableau.field).name = "SectionItem"];
	}
}
```

#### Output with prefix
```
// demo_test.proto
message DemoActivity{
	map<uint32, Activity> activity_map = 1 [(key) = "ActivityID"];
	message Activity {
		uint32 activity_id= 1 [(tableau.field).name = "ActivityID"];
		string activity_name = 2 [(tableau.field).name = "ActivityName"];
		map<uint32, Chapter> chapter_map = 3 [(tableau.field).key = "ChapterID"];
	}
	message Chapter {
		uint32 chapter_id= 1 [(tableau.field).name = "ChapterID"];
		string chapter_name = 2 [(tableau.field).name = "ChapterName"];
		repeated Section section_list = 3 [(tableau.field).layout = LAYOUT_VERTICAL];
	}
	message Section {
		uint32 section_id= 1 [(tableau.field).name = "SectionID"];
		string section_name = 2 [(tableau.field).name = "SectionName"];
		repeated Item section_item_list = 3 [(tableau.field).name = "SectionItem"];
	}
}
```

### Incell

workbook: `(AliasTest)DemoTest`, worksheet: `(Env)Environment`

| ID     | Name   | IncellMessage                         | IncellList | IncellMap         | IncellMessageList            | IncellMessageMap                       |
| ------ | ------ | ------------------------------------- | ---------- | ----------------- | ---------------------------- | -------------------------------------- |
| uint32 | string | {int32 id,string desc,int32 value}Msg | []int32    | map<int32,string> | []{int32 id,string desc}Elem | map<int32,Value{int32 id,string desc}> |
| 1      | Earth  | 1,desc,100                            | 1,2,3      | 1:hello,2:world   | {1,hello},{2,world}          | 1:{1,hello},2:{2,world}                |

#### IncellMessage
Syntax: *TODO: EBNF*
Type: message type
Value: comma seperated field values, e.g.: `1,desc,100`
Rules:
| Default Type | Value                      |
| ------------ | -------------------------- |
| int32        | can be parsed as number    |
| string       | cannot be parsed as number |

#### IncellList
Syntax: `[]Type`
Type: any scalar type
Value: comma seperated list items, e.g.: `1,2,3`

#### IncellMap
Syntax: `map<Type,Type>`
Type: any scalar type
Value: comma seperated key-value pairs, and key-value is seperated by colon. e.g.: `1:hello,2:world`

#### IncellMessageList
*TODO*

#### IncellMessageMap
*TODO*

#### Output
```
// demo_test.proto
message Env {
	uint32 ID = 1 [(tableau.field).name = "ID"];
	string name = 2 [(tableau.field).name = "Name"];
	Msg incell_message= 3 [(tableau.field).name = "IncellMessage"];
	repeated int32 incell_list= 4 [(tableau.field).name = "IncellList"];
	map<int32, string> incell_map = 5 [(tableau.field).name = "IncellMap"];
	repeated Elem incell_message_list= 6 [(tableau.field).name = "IncellMessageList"];
    map<int32, Value> incell_message_map = 7 [(tableau.field).name = "IncellMessageMap"];

    // defaut name: field + <tagid>
	message Msg {
		int32 id = 1;
		string desc= 2; 
		int32 value= 3;
	}
    message Elem {
		int32 id = 1;
		string desc= 2;
	}
    message Value {
		int32 id = 1;
		string desc= 2;
	}
}
```

- Incell message: comma seperated sequence: `{TYPE [NAME],TYPE [NAME]}`, NAME is optional, and will be auto generated as `field + <tagid>` if not specified.
- Incell list: `[]TYPE`, TYPE must be scalar type.
- Incell map: `map[KEY]VALUE`, KEY and VALUE must be scalar types.
- Incell message list: `[]TYPE`, TYPE must be message type.
- Incell message map: `map[KEY]VALUE`, KEY is scalar, and VALUE must be message type.

## Protoconf -> Workbook
[TODO]