# Tutorial

## A Simple Example

Workbook: "club.xlsx", Worksheet: "member"

| ID  | Name  | Age | Description                        |
| --- | ----- | --- | ---------------------------------- |
| 1   | Bob   | 24  | A high school student.             |
| 2   | Lily  | 19  | A famous female model.             |
| 3   | Blues | 35  | A famous action star in the world. |

```proto3
option (workbook) = "club.xlsx"
message MemberConf {
  option (worksheet) = "member";
  map<int32, Member> member_map = 1 [ (key) = "ID" ];

  message Member {
    int32 id = 1 [ (caption) = "ID" ];
    string name = 2 [ (caption) = "Name" ];
    int32 age = 3 [ (caption) = "Age" ];
    string desc = 5 [ (caption) = "Description" ];
  }
}
```