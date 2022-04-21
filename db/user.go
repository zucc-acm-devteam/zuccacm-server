package db

import (
	"context"
	"database/sql"
	"sort"

	log "github.com/sirupsen/logrus"
)

type User struct {
	Username string `db:"username" json:"username"`
	Nickname string `db:"nickname" json:"nickname"`
	CfRating int    `db:"cf_rating" json:"cf_rating"`
	IsEnable bool   `db:"is_enable" json:"is_enable"`
	IsAdmin  bool   `db:"is_admin" json:"is_admin"`
	IdCard   string `db:"id_card" json:"id_card"`
	Phone    string `db:"phone" json:"phone"`
	QQ       string `db:"qq" json:"qq"`
	TShirt   string `db:"t_shirt" json:"t_shirt"`
}

// GetUserByUsername return nil when user not found
func GetUserByUsername(ctx context.Context, username string) (ret *User) {
	ret = &User{}
	err := instance.GetContext(ctx, ret, "SELECT * FROM user WHERE username = ?", username)
	if err == sql.ErrNoRows {
		log.WithField("username", username).Warn("user not found")
		ret = nil
		err = nil
	}
	if err != nil {
		panic(err)
	}
	return
}

func AddUser(ctx context.Context, user User) {
	tx := instance.MustBeginTx(ctx, nil)
	defer tx.Rollback()

	mustNamedExecTx(tx, ctx, addUserSQL, user)
	team := Team{
		Name:     user.Nickname,
		IsEnable: user.IsEnable,
		IsSelf:   true,
	}
	ret := mustNamedExecTx(tx, ctx, addTeamSQL, team)
	tmp, err := ret.LastInsertId()
	if err != nil {
		panic(err)
	}
	team.Id = int(tmp)
	mustNamedExecTx(tx, ctx, addTeamUserRelSQL, TeamUser{team.Id, user.Username})
	mustCommit(tx)
}

func UpdUser(ctx context.Context, user User) {
	team := GetTeamBySelf(ctx, user.Username)
	team.Name = user.Nickname
	tx := instance.MustBeginTx(ctx, nil)
	defer tx.Rollback()
	mustNamedExecTx(tx, ctx, updUserSQL, user)
	mustNamedExecTx(tx, ctx, updTeamSQL, team)
	mustCommit(tx)
}

func UpdUserAdmin(ctx context.Context, user User) {
	mustNamedExec(ctx, updUserAdminSQL, user)
}

func UpdUserEnable(ctx context.Context, user User) {
	team := GetTeamBySelf(ctx, user.Username)
	team.IsEnable = user.IsEnable
	tx := instance.MustBeginTx(ctx, nil)
	defer tx.Rollback()
	mustNamedExec(ctx, updUserEnableSQL, user)
	mustNamedExec(ctx, updTeamEnableSQL, team)
	mustCommit(tx)
}

type Award struct {
	Username string `json:"username" db:"username"`
	Medal    int    `json:"medal" db:"medal"`
	Award    string `json:"award" db:"award"`
	XcpcId   int    `json:"xcpc_id" db:"xcpc_id"`
}

// GetAwardsByUsername return awards of 1 user
func GetAwardsByUsername(ctx context.Context, username string) []Award {
	query := getAwardsSQL + " AND user.username=? ORDER BY xcpc_date"
	ret := make([]Award, 0)
	mustSelect(ctx, &ret, query, username)
	return ret
}

// GetAwardsAll return awards of all users
// only return enable users if isEnable=true
func GetAwardsAll(ctx context.Context, isEnable bool) []Award {
	query := getAwardsSQL
	if isEnable {
		query += " AND is_enable=true"
	}
	query += " ORDER BY xcpc_date"
	ret := make([]Award, 0)
	mustSelect(ctx, &ret, query)
	return ret
}

type userGroup struct {
	GroupId   int    `json:"group_id" db:"group_id"`
	GroupName string `json:"group_name" db:"group_name"`
	Users     []User `json:"users"`
}

// GetOfficialGroups return official groups without users
// Official groups are groups which is_grade=true, such as 2018, 2019
func GetOfficialGroups(ctx context.Context) []userGroup {
	query := `SELECT group_id, group_name FROM team_group WHERE is_grade`
	ret := make([]userGroup, 0)
	mustSelect(ctx, &ret, query)
	return ret
}

// GetOfficialUsers return official groups with users
// return all users if is_enable=false
// groups with no user will be ignored
// each user can be in at most 1 group at a time
// official group should only contain teams with is_self=true
func GetOfficialUsers(ctx context.Context, isEnable bool) []userGroup {
	grp := make(map[int]*userGroup)
	groups := GetOfficialGroups(ctx)
	for _, row := range groups {
		grp[row.GroupId] = &userGroup{
			GroupId:   row.GroupId,
			GroupName: row.GroupName,
			Users:     make([]User, 0),
		}
	}
	query := `SELECT user.username AS username, nickname, cf_rating, is_enable, is_admin, team_group.group_id AS group_id
FROM user, team_group_rel, team_group, team_user_rel
WHERE user.username = team_user_rel.username AND team_user_rel.team_id = team_group_rel.team_id
AND team_group.group_id = team_group_rel.group_id AND is_grade`
	if isEnable {
		query += " AND is_enable"
	}
	query += " ORDER BY group_name DESC"
	var data []struct {
		Username string `db:"username"`
		Nickname string `db:"nickname"`
		CfRating int    `db:"cf_rating"`
		IsEnable bool   `db:"is_enable"`
		IsAdmin  bool   `db:"is_admin"`
		GroupId  int    `db:"group_id"`
	}
	mustSelect(ctx, &data, query)
	for _, x := range data {
		grp[x.GroupId].Users = append(grp[x.GroupId].Users, User{
			Username: x.Username,
			Nickname: x.Nickname,
			CfRating: x.CfRating,
			IsEnable: x.IsEnable,
			IsAdmin:  x.IsAdmin,
		})
	}
	ret := make([]userGroup, 0)
	for _, v := range grp {
		if len(v.Users) > 0 {
			ret = append(ret, *v)
		}
	}
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].GroupName > ret[j].GroupName
	})
	return ret
}