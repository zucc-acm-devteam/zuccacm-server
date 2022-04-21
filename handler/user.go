package handler

import (
	"net/http"
	"sort"
	"time"

	"zuccacm-server/db"
	"zuccacm-server/utils"
)

var userRouter = Router.PathPrefix("/user").Subrouter()

func init() {
	userRouter.HandleFunc("/add", adminOnly(addUser)).Methods("POST")
	userRouter.HandleFunc("/upd", updUser).Methods("POST")
	userRouter.HandleFunc("/upd_admin", adminOnly(updUserAdmin)).Methods("POST")
	userRouter.HandleFunc("/upd_enable", adminOnly(updUserEnable)).Methods("POST")
	userRouter.HandleFunc("/{username}", getUser).Methods("GET")
	userRouter.HandleFunc("/{username}/accounts", getUserAccounts).Methods("GET")
	userRouter.HandleFunc("/{username}/accounts/upd", updUserAccount).Methods("POST")
	userRouter.HandleFunc("/{username}/submissions", getUserSubmissions).Methods("GET")
	userRouter.HandleFunc("/{username}/contests", getUserContests).Methods("GET")

	Router.HandleFunc("/users", getUsers).Methods("GET")
}

func addUser(w http.ResponseWriter, r *http.Request) {
	var user db.User
	decodeParamVar(r, &user)
	db.AddUser(r.Context(), user)
	msgResponse(w, http.StatusOK, "添加用户成功")
}

// upd nickname, id_card, phone, qq, t_shirt
func updUser(w http.ResponseWriter, r *http.Request) {
	var user db.User
	decodeParamVar(r, &user)
	now := getCurrentUser(r)
	if !now.IsAdmin && now.Username != user.Username {
		panic(utils.ErrForbidden)
	}
	db.UpdUser(r.Context(), user)
	msgResponse(w, http.StatusOK, "修改用户信息成功")
}

func updUserAdmin(w http.ResponseWriter, r *http.Request) {
	var user db.User
	decodeParamVar(r, &user)
	db.UpdUserAdmin(r.Context(), user)
	msgResponse(w, http.StatusOK, "修改用户权限成功")
}

func updUserEnable(w http.ResponseWriter, r *http.Request) {
	var user db.User
	decodeParamVar(r, &user)
	db.UpdUserEnable(r.Context(), user)
	msgResponse(w, http.StatusOK, "修改用户状态成功")
}

func updUserAccount(w http.ResponseWriter, r *http.Request) {
	var account db.Account
	decodeParamVar(r, &account)
	db.UpdAccount(r.Context(), account)
	msgResponse(w, http.StatusOK, "修改用户账号成功")
}

func getUserAccounts(w http.ResponseWriter, r *http.Request) {
	type account struct {
		OjId    int    `json:"oj_id" db:"oj_id"`
		OjName  string `json:"oj_name" db:"oj_name"`
		Account string `json:"account" db:"account"`
	}
	ctx := r.Context()
	username := getParamURL(r, "username")

	oj := db.GetOjAll(ctx)
	data := make([]account, len(oj))
	mp := make(map[int]int)
	for i, x := range oj {
		data[i] = account{
			OjId:   x.OjId,
			OjName: x.OjName,
		}
		mp[x.OjId] = i
	}
	ac := db.GetAccountByUsername(ctx, username)
	for _, x := range ac {
		data[mp[x.OjId]].Account = x.Account
	}
	dataResponse(w, data)
}

func getUserSubmissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := getParamURL(r, "username")
	begin := getParamDateRequired(r, "begin_time")
	end := getParamDateRequired(r, "end_time").Add(time.Hour * 24).Add(time.Second * -1)
	submissions := db.GetSubmissionByUsername(ctx, username, begin, end)
	n := utils.SubDays(begin, end) + 1
	data := make([]int, n)
	for _, s := range submissions {
		i := utils.SubDays(begin, time.Time(s.CreateTime))
		data[i]++
	}
	dataResponse(w, data)
}

func getUserContests(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		ContestId      int             `json:"contest_id"`
		ContestName    string          `json:"contest_name"`
		StartTime      db.Datetime     `json:"start_time"`
		Duration       int             `json:"duration"`
		Solved         int             `json:"solved"`
		Problems       []db.Problem    `json:"problems"`
		ProblemResults []problemResult `json:"problem_results"`
	}
	data := struct {
		MaxProblems int   `json:"max_problems"`
		Contests    []Row `json:"contests"`
	}{0, make([]Row, 0)}
	ctx := r.Context()
	username := getParamURL(r, "username")
	begin := getParamDate(r, "begin_time", defaultBeginTime)
	end := getParamDate(r, "end_time", defaultEndTime).Add(time.Hour * 24).Add(time.Second * -1)
	groupId := getParamInt(r, "group_id", 0)
	contests := db.GetContestsByUser(ctx, username, begin, end, groupId)
	for _, c := range contests {
		data.Contests = append(data.Contests, Row{
			ContestId:      c.Id,
			ContestName:    c.Name,
			StartTime:      c.StartTime,
			Duration:       c.Duration,
			Problems:       c.Problems,
			ProblemResults: make([]problemResult, len(c.Problems)),
		})
		data.MaxProblems = utils.Max(data.MaxProblems, len(c.Problems))
	}
	submissions := db.GetSubmissionByUsername(ctx, username, defaultBeginTime, defaultEndTime)
	type Key struct {
		OjId int
		Pid  string
	}
	mp := make(map[Key][]submissionInfo)
	for _, s := range submissions {
		key := Key{s.OjId, s.Pid}
		mp[key] = append(mp[key], submissionInfo{s.IsAccepted, s.CreateTime})
	}
	for i, c := range data.Contests {
		for j, p := range c.Problems {
			data.Contests[i].ProblemResults[j] = calcProblemResult(mp[Key{p.OjId, p.Pid}], c.StartTime, c.Duration)
			if data.Contests[i].ProblemResults[j].AcceptedTime != -1 {
				data.Contests[i].Solved++
			}
		}
	}
	dataResponse(w, data)
}

// getUser return user's basic info and awards
func getUser(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Username string     `json:"username"`
		Nickname string     `json:"nickname"`
		CfRating int        `json:"cf_rating"`
		IsEnable bool       `json:"is_enable"`
		IsAdmin  bool       `json:"is_admin"`
		Medals   [3]int     `json:"medals"`
		Awards   []db.Award `json:"awards"`
	}
	username := getParamURL(r, "username")
	ctx := r.Context()

	u := db.GetUserByUsername(ctx, username)
	data.Username = u.Username
	data.Nickname = u.Nickname
	data.CfRating = u.CfRating
	data.IsEnable = u.IsEnable
	data.IsAdmin = u.IsAdmin
	data.Awards = db.GetAwardsByUsername(ctx, username)
	for _, x := range data.Awards {
		if x.Medal > 0 {
			data.Medals[x.Medal-1]++
		}
	}
	dataResponse(w, data)
}

// getUsers get all official users if is_enable=false (default is true)
func getUsers(w http.ResponseWriter, r *http.Request) {
	type user struct {
		Username string   `json:"username"`
		Nickname string   `json:"nickname"`
		CfRating int      `json:"cf_rating"`
		Awards   []string `json:"awards"`
		Medals   [3]int   `json:"medals"`
	}
	type group struct {
		GroupId   int    `json:"group_id"`
		GroupName string `json:"group_name"`
		Users     []user `json:"users"`
	}
	isEnable := getParamBool(r, "is_enable", false)
	ctx := r.Context()

	mpUser := make(map[string]*user)
	mpGroup := make(map[int]*group)
	userGroup := db.GetOfficialUsers(ctx, isEnable)
	for _, x := range userGroup {
		mpGroup[x.GroupId] = &group{
			GroupId:   x.GroupId,
			GroupName: x.GroupName,
			Users:     make([]user, 0),
		}
		for _, u := range x.Users {
			mpUser[u.Username] = &user{
				Username: u.Username,
				Nickname: u.Nickname,
				CfRating: u.CfRating,
				Awards:   make([]string, 0),
			}
		}
	}
	userAward := db.GetAwardsAll(ctx, isEnable)
	for _, x := range userAward {
		if x.Medal > 0 {
			mpUser[x.Username].Medals[x.Medal-1]++
		}
		if len(x.Award) > 0 {
			mpUser[x.Username].Awards = append(mpUser[x.Username].Awards, x.Award)
		}
	}
	for _, x := range userGroup {
		for _, u := range x.Users {
			mpGroup[x.GroupId].Users = append(mpGroup[x.GroupId].Users, *mpUser[u.Username])
		}
	}
	var data []group
	for _, x := range mpGroup {
		data = append(data, *x)
	}
	sort.SliceStable(data, func(i, j int) bool {
		return data[i].GroupName > data[j].GroupName
	})
	for k := range data {
		sort.SliceStable(data[k].Users, func(i, j int) bool {
			return data[k].Users[i].Username < data[k].Users[j].Username
		})
	}
	dataResponse(w, data)
}