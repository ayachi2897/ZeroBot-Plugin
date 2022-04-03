// Package bilibili 查询b站用户信息
package bilibili

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"os"
	"sort"
	"strconv"
	"time"

	control "github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/file"
	"github.com/FloatTech/zbputils/img"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/FloatTech/zbputils/img/writer"
	"github.com/FloatTech/zbputils/web"
	"github.com/fogleman/gg"
	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var engine = control.Register("bilibili", &control.Options{
	DisableOnDefault: false,
	Help: "bilibili\n" +
		"- >vup info [xxx]\n" +
		"- >user info [xxx]\n" +
		"- 查成分 [xxx]\n" +
		"- 设置b站cookie SESSDATA=82da790d,1663822823,06ecf*31\n" +
		"- 更新vup",
	PublicDataFolder: "Bilibili",
})

// 查成分的
func init() {
	cachePath := engine.DataFolder() + "cache/"
	dbfile := engine.DataFolder() + "bilibili.db"
	go func() {
		_ = os.MkdirAll(cachePath, 0755)
		_, _ = file.GetLazyData(dbfile, false, false)
		vdb = initialize(dbfile)
	}()

	engine.OnRegex(`^>user info\s?(.{1,25})$`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			keyword := ctx.State["regex_matched"].([]string)[1]
			uidRes, err := search(keyword)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			id := strconv.FormatInt(uidRes[0].Mid, 10)
			follwings, err := followings(id)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
			}
			ctx.SendChain(message.Text(
				"search: ", uidRes[0].Mid, "\n",
				"name: ", uidRes[0].Uname, "\n",
				"sex: ", []string{"", "男", "女", "未知"}[uidRes[0].Gender], "\n",
				"sign: ", uidRes[0].Usign, "\n",
				"level: ", uidRes[0].Level, "\n",
				"follow: ", follwings,
			))
		})

	engine.OnRegex(`^>vup info\s?(.{1,25})$`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			keyword := ctx.State["regex_matched"].([]string)[1]
			res, err := search(keyword)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			id := strconv.FormatInt(res[0].Mid, 10)
			// 获取详情
			fo, err := fansapi(id)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Text(
				"search: ", fo.Mid, "\n",
				"名字: ", fo.Uname, "\n",
				"当前粉丝数: ", fo.Follower, "\n",
				"24h涨粉数: ", fo.Rise, "\n",
				"视频投稿数: ", fo.Video, "\n",
				"直播间id: ", fo.Roomid, "\n",
				"舰队: ", fo.GuardNum, "\n",
				"直播总排名: ", fo.AreaRank, "\n",
				"数据来源: ", "https://vtbs.moe/detail/", fo.Mid, "\n",
				"数据获取时间: ", time.Now().Format("2006-01-02 15:04:05"),
			))
		})

	engine.OnRegex(`^查成分\s?(.{1,25})$`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			keyword := ctx.State["regex_matched"].([]string)[1]
			searchRes, err := search(keyword)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			id := strconv.FormatInt(searchRes[0].Mid, 10)
			today := time.Now().Format("20060102")
			drawedFile := cachePath + id + today + "vupLike.png"
			if file.IsExist(drawedFile) {
				ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
				return
			}
			u, err := card(id)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			vups, err := vdb.filterVup(u.Attentions)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			vupLen := len(vups)
			medals, err := medalwall(id)
			sort.Sort(medalSlice(medals))
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
			}
			frontVups := make([]vup, 0)
			medalMap := make(map[int64]medal)
			for _, v := range medals {
				up := vup{
					Mid:   v.Mid,
					Uname: v.Uname,
				}
				frontVups = append(frontVups, up)
				medalMap[v.Mid] = v
			}
			vups = append(vups, frontVups...)
			copy(vups[len(frontVups):], vups)
			copy(vups, frontVups)
			for i := len(frontVups); i < len(vups); i++ {
				if _, ok := medalMap[vups[i].Mid]; ok {
					vups = append(vups[:i], vups[i+1:]...)
					i--
				}
			}
			facePath := cachePath + id + "vupFace.png"
			initFacePic(facePath, u.Face)
			var backX int
			var backY int
			back, err := gg.LoadImage(facePath)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				backX = 500
				backY = 500
			} else {
				back = img.Limit(back, 500, 500)
				backX = back.Bounds().Size().X
				backY = back.Bounds().Size().Y
			}
			if len(vups) > 50 {
				ctx.SendChain(message.Text(u.Name + "关注的up主太多了，只展示前50个up"))
				vups = vups[:50]
			}
			canvas := gg.NewContext(backX*3, int(float64(backY)*(1.1+float64(len(vups))/3)))
			fontSize := float64(backX) * 0.1
			canvas.SetColor(color.White)
			canvas.Clear()
			if back != nil {
				canvas.DrawImage(back, 0, 0)
			}
			canvas.SetColor(color.Black)
			if err = canvas.LoadFontFace(text.BoldFontFile, fontSize); err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			sl, _ := canvas.MeasureString("好")
			length, h := canvas.MeasureString(u.Mid)
			n, _ := canvas.MeasureString(u.Name)
			canvas.DrawString(u.Name, float64(backX)*1.1, float64(backY)/3-h)
			canvas.DrawRoundedRectangle(float64(backX)*1.2+n-length*0.1, float64(backY)/3-h*2.5, length*1.2, h*2, fontSize*0.2)
			canvas.SetRGB255(221, 221, 221)
			canvas.Fill()
			canvas.SetColor(color.Black)
			canvas.DrawString(u.Mid, float64(backX)*1.2+n, float64(backY)/3-h)
			canvas.DrawString(fmt.Sprintf("粉丝：%d", u.Fans), float64(backX)*1.1, float64(backY)/3*2-2.5*h)
			canvas.DrawString(fmt.Sprintf("关注：%d", len(u.Attentions)), float64(backX)*2, float64(backY)/3*2-2.5*h)
			canvas.DrawString(fmt.Sprintf("管人痴成分：%.2f%%（%d/%d）", float64(vupLen)/float64(len(u.Attentions))*100, vupLen, len(u.Attentions)), float64(backX)*1.1, float64(backY)-4*h)
			canvas.DrawString("日期："+time.Now().Format("2006-01-02"), float64(backX)*1.1, float64(backY)-h)
			for i, v := range vups {
				if i%2 == 1 {
					canvas.SetRGB255(245, 245, 245)
					canvas.DrawRectangle(0, float64(backY)*1.1+float64(i)*float64(backY)/3, float64(backX*3), float64(backY)/3)
					canvas.Fill()
				}
				canvas.SetColor(color.Black)
				nl, _ := canvas.MeasureString(v.Uname)
				canvas.DrawString(v.Uname, float64(backX)*0.1, float64(backY)*1.1+float64(i+1)*float64(backY)/3-2*h)
				ml, _ := canvas.MeasureString(strconv.FormatInt(v.Mid, 10))
				canvas.DrawRoundedRectangle(nl-0.1*ml+float64(backX)*0.2, float64(backY)*1.1+float64(i+1)*float64(backY)/3-h*3.5, ml*1.2, h*2, fontSize*0.2)
				canvas.SetRGB255(221, 221, 221)
				canvas.Fill()
				canvas.SetColor(color.Black)
				canvas.DrawString(strconv.FormatInt(v.Mid, 10), nl+float64(backX)*0.2, float64(backY)*1.1+float64(i+1)*float64(backY)/3-2*h)
				if m, ok := medalMap[v.Mid]; ok {
					mnl, _ := canvas.MeasureString(m.MedalName)
					grad := gg.NewLinearGradient(nl+ml-sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-3.5*h, nl+ml+mnl+sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-1.5*h)
					r, g, b := int2rbg(m.MedalColorStart)
					grad.AddColorStop(0, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
					r, g, b = int2rbg(m.MedalColorEnd)
					grad.AddColorStop(1, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
					canvas.SetFillStyle(grad)
					canvas.SetLineWidth(4)
					canvas.MoveTo(nl+ml-sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-3.5*h)
					canvas.LineTo(nl+ml+mnl+sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-3.5*h)
					canvas.LineTo(nl+ml+mnl+sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-1.5*h)
					canvas.LineTo(nl+ml-sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-1.5*h)
					canvas.ClosePath()
					canvas.Fill()
					canvas.SetColor(color.White)
					canvas.DrawString(m.MedalName, nl+ml+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-2*h)
					r, g, b = int2rbg(m.MedalColorBorder)
					canvas.SetRGB255(int(r), int(g), int(b))
					canvas.DrawString(strconv.FormatInt(m.Level, 10), nl+ml+mnl+sl+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-2*h)
					mll, _ := canvas.MeasureString(strconv.FormatInt(m.Level, 10))
					canvas.SetLineWidth(4)
					canvas.MoveTo(nl+ml-sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-3.5*h)
					canvas.LineTo(nl+ml+mnl+mll+sl/2+float64(backX)*0.5, float64(backY)*1.1+float64(i+1)*float64(backY)/3-3.5*h)
					canvas.LineTo(nl+ml+mnl+mll+sl/2+float64(backX)*0.5, float64(backY)*1.1+float64(i+1)*float64(backY)/3-1.5*h)
					canvas.LineTo(nl+ml-sl/2+float64(backX)*0.4, float64(backY)*1.1+float64(i+1)*float64(backY)/3-1.5*h)
					canvas.ClosePath()
					canvas.Stroke()
				}
			}
			f, err := os.Create(drawedFile)
			if err != nil {
				log.Errorln("[bilibili]", err)
				data, cl := writer.ToBytes(canvas.Image())
				ctx.SendChain(message.ImageBytes(data))
				cl()
				return
			}
			_, err = writer.WriteTo(canvas.Image(), f)
			_ = f.Close()
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Image("file:///" + file.BOTPATH + "/" + drawedFile))
		})

	engine.OnRegex(`^设置b站cookie?\s+(.{1,100})$`, zero.SuperUserPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			cookie := ctx.State["regex_matched"].([]string)[1]
			err := vdb.setBilibiliCookie(cookie)
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Text("成功设置b站cookie为" + cookie))
		})

	engine.OnFullMatch("更新vup", zero.SuperUserPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			ctx.SendChain(message.Text("少女祈祷中..."))
			updateVup()
			ctx.SendChain(message.Text("vup已更新"))
		})
}

func initFacePic(filename, faceURL string) {
	if file.IsNotExist(filename) {
		data, err := web.GetData(faceURL)
		if err != nil {
			log.Errorln("[bilibili]", err)
		}
		err = os.WriteFile(filename, data, 0666)
		if err != nil {
			log.Errorln("[bilibili]", err)
		}
	}
}

func int2rbg(t int64) (int64, int64, int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(t))
	b, g, r := int64(buf[0]), int64(buf[1]), int64(buf[2])
	return r, g, b
}