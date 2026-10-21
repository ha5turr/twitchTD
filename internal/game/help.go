package game

import (
	"fmt"
	"strings"
	"time"
)

// helpCooldown keeps !помощь from flooding chat: each topic answers at most
// once a minute for everyone, since the whole chat sees the answer anyway.
const helpCooldown = time.Minute

var helpTopics = map[string]string{
	"": "", "стройка": "build", "строить": "build", "build": "build", "башни": "build",
	"таверна": "tavern", "tavern": "tavern", "кости": "tavern",
	"души": "souls", "souls": "souls", "ачивки": "souls", "мемы": "souls",
	"клоун": "clown", "clown": "clown",
}

func (g *Game) cmdHelp(c *Chatter, args []string) {
	topic := ""
	if len(args) > 0 {
		topic = helpTopics[strings.ToLower(args[0])]
	}
	if topic == "clown" && !g.sdfOut() {
		topic = "" // the clown stays a secret until his boss shows up
	}
	now := g.Now()
	if now.Sub(g.helpAt[topic]) < helpCooldown {
		return
	}
	g.helpAt[topic] = now
	g.reply(c.Name, g.helpText(topic))
}

func (g *Game) helpText(topic string) string {
	switch topic {
	case "build":
		return fmt.Sprintf("стройка: !b клетка тип — наёмник идёт строить. Рядом с дорогой: arrow, magic, fire, shrine, drums, curse, totem; на дорогу: trap, wall. "+
			"!up клетка — улучшить до 5 ур., !sell клетка — продать за %d%%. Лимит построек сейчас %d (+1 за каждого босса, до %d), класс (!class) усиливает свою постройку на %d%%.",
			g.Cfg.SellRefundPct, g.TowerLimit(), max(g.Cfg.TowerLimitMax, g.Cfg.TowerLimit), int(g.Cfg.ClassBonus*100))
	case "tavern":
		return "таверна: !drink [эль|медовуха|зелье] — выпить, !кости 50 — ставка в кости, !кости принять / перебить N / отмена, " +
			"!class — сменить класс (раз за игру), !link — привязать донаты."
	case "souls":
		return fmt.Sprintf("души копятся между играми: в конце игры за остаток золота и вклад в бой, и за ачивки. "+
			"!души — баланс, !ачивки — твои ачивки, !мемы N — заявка на N мемкоинов (%d душ за мемкоин), стример начислит вручную.",
			g.Cfg.SoulsPerMemecoin)
	case "clown":
		return "клоун: !b клетка clown — бьёт только босса-гостя, по всей карте, один на игрока, после волны вернётся золото."
	}
	return "команды: !play — войти в игру, !leave — выйти, !b C4 arrow — построить, !up C4 — улучшить, !sell C4 — продать, !towers — постройки, !me — твой наёмник, " +
		"!drink — выпить, !кости 50 — кости, !души — души. Подробнее: !помощь стройка | таверна | души"
}
