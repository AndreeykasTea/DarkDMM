package dmaccess

import "regexp"

// Russian names of common access descriptions (Paradise/TG). Unknown ones are shown as in the code.
var accessNamesRu = map[string]string{
	"Cargo Bay": "Карго",

	// Foundation-19.
	"Ascent Materiel":               "Материалы Ascent",
	"Chief Science Officer":         "Директор исследований",
	"Civilian Comms":                "Связь: гражданская",
	"Code Black":                    "Код чёрный",
	"Code Blue":                     "Код синий",
	"Code Gold":                     "Код золотой",
	"Code Green":                    "Код зелёный",
	"Code Grey":                     "Код серый",
	"Code Orange":                   "Код оранжевый",
	"Code Silver":                   "Код серебряный",
	"Code White":                    "Код белый",
	"Code Yellow":                   "Код жёлтый",
	"Command":                       "Командование",
	"Command Comms":                 "Связь: командование",
	"Counselor's Office":            "Кабинет психолога",
	"Crate cash":                    "Ящик с деньгами",
	"D-Class Botany":                "Класс D: ботаника",
	"D-Class Janitorial":            "Класс D: уборка",
	"D-Class Kitchen":               "Класс D: кухня",
	"D-Class Luxury":                "Класс D: люкс",
	"D-Class Medical":               "Класс D: медпункт",
	"D-Class Mining":                "Класс D: шахта",
	"Engine Room":                   "Машинное отделение",
	"Engineering Comms":             "Связь: инженеры",
	"Fax Machines":                  "Факсы",
	"Foundation Advisor":            "Советник Фонда",
	"Internal Affairs":              "Внутренний трибунал",
	"Keycard Authentication Access": "Авторизация по ключ-картам",
	"Logistics Comms":               "Связь: логистика",
	"MTF":                           "МОГ",
	"Medical Comms":                 "Связь: медики",
	"Medical Equipment":             "Медицинское оборудование",
	"Merchant":                      "Торговец",
	"Primary Network":               "Основная сеть",
	"Research Comms":                "Связь: наука",
	"Research Labs":                 "Лаборатории",
	"Security Comms":                "Связь: охрана",
	"Security Equipment":            "Оборудование охраны",
	"Syndicate":                     "Синдикат",
	"Synthetic":                     "Синтетик",

	"Cargo Bot Delivery":    "Доставка ботами",
	"Security":              "Охрана",
	"Holding Cells":         "Камеры",
	"Courtroom":             "Зал суда",
	"Forensics":             "Криминалистика",
	"Medical":               "Медбей",
	"Genetics Lab":          "Генетика",
	"Morgue":                "Морг",
	"R&D Lab":               "РнД",
	"Toxins Lab":            "Токсины",
	"Chemistry Lab":         "Химлаб",
	"Research Director":     "Директор исследований",
	"Bar":                   "Бар",
	"Custodial Closet":      "Каморка уборщика",
	"Engineering":           "Инженерия",
	"Power Equipment":       "Энергооборудование",
	"Maintenance":           "Техтоннели",
	"External Airlocks":     "Внешние шлюзы",
	"Emergency Storage":     "Аварийные склады",
	"ID Computer":           "Консоль ID-карт",
	"AI Upload":             "Загрузка законов ИИ",
	"Teleporter":            "Телепортер",
	"EVA":                   "EVA",
	"Bridge":                "Мостик",
	"Captain":               "Капитан",
	"Personal Lockers":      "Личные шкафчики",
	"Chapel Office":         "Офис церкви",
	"Technical Storage":     "Техсклад",
	"Atmospherics":          "Атмосферная",
	"Crematorium":           "Крематорий",
	"Armory":                "Оружейная",
	"Construction Areas":    "Стройзоны",
	"Kitchen":               "Кухня",
	"Hydroponics":           "Гидропоника",
	"Library":               "Библиотека",
	"Law Office":            "Юрист",
	"Robotics":              "Робототехника",
	"Virology":              "Вирусология",
	"Psychiatrist's Office": "Кабинет психиатра",
	"Chief Medical Officer": "Главврач",
	"Quartermaster":         "Квартирмейстер",
	"Clown's Office":        "Комната клоуна",
	"Mime's Office":         "Комната мима",
	"Surgery":               "Хирургия",
	"Theatre":               "Театр",
	"Manufacturing":         "Производство",
	"Science":               "Наука",
	"Mining":                "Шахтёры",
	"Bitrunning Den":        "Битраннинг",
	"Mining Office":         "Офис шахтёров",
	"Cargo Office":          "Офис карго",
	"Mint":                  "Монетный двор",
	"Mint Vault":            "Хранилище монетного двора",
	"Main Vault":            "Главное хранилище",
	"Mining EVA":            "EVA шахтёров",
	"Xenobiology Lab":       "Ксенобиология",
	"Xenoarchaeology":       "Ксеноархеология",
	"Head of Personnel":     "Глава персонала",
	"Head of Security":      "Глава службы безопасности",
	"Chief Engineer":        "Старший инженер",
	"RC Announcements":      "Объявления с консолей",
	"Keycode Auth. Device":  "Устройство аутентификации",
	"Telecommunications":    "Телекоммуникации",
	"Network Access":        "Доступ к сети",
	"Gateway":               "Врата",
	"Brig":                  "Бриг",
	"Blueshield":            "Блюшилд",
	"Nanotrasen Rep.":       "Представитель NT",
	"Paramedic":             "Парамедик",
	"Mechanic Workshop":     "Мастерская механика",
	"Security Pod Pilot":    "Пилот СБ",
	"Magistrate":            "Магистрат",
	"Mineral Storage":       "Склад минералов",
	"AI Satellite":          "Спутник ИИ",
	"Weapon Permit":         "Разрешение на оружие",
	"Investor":              "Инвестор",
	"General Access":        "Общий доступ",
	"Living Quarters":       "Жилые помещения",
	"Storage":               "Склад",
	"Shuttles":              "Шаттлы",
	"Special Ops":           "Спецоперации",
	"Special Ops Commander": "Командир спецопераций",
	"Black Ops":             "Чёрные операции",
	"Thunderdome":           "Тандердом",
	"Commander":             "Командир",
	"Syndicate Operative":   "Оперативник Синдиката",
}

// Department words of Foundation-19 like accesses: "Security Level 2" -> "Охрана, уровень 2".
var levelDepartmentsRu = map[string]string{
	"Security":       "Охрана",
	"Science":        "Наука",
	"Medical":        "Медицина",
	"Engineering":    "Инженерия",
	"Admin":          "Администрация",
	"Administration": "Администрация",
	"Logistics":      "Логистика",
	"Supply":         "Снабжение",
	"Command":        "Командование",
	"Service":        "Сервис",
}

var reLevelDesc = regexp.MustCompile(`^(\w+) Level (\d+)$`)

// translateLevel translates "<Department> Level N" descriptions.
var reContainmentDesc = regexp.MustCompile(`^(SCP-[\w-]+) Containment$`)

func translateLevel(desc string) (string, bool) {
	// Foundation-19: "SCP-012 Containment".
	if m := reContainmentDesc.FindStringSubmatch(desc); m != nil {
		return "Содержание " + m[1], true
	}
	m := reLevelDesc.FindStringSubmatch(desc)
	if m == nil {
		return "", false
	}
	dep, ok := levelDepartmentsRu[m[1]]
	if !ok {
		return "", false
	}
	return dep + ", уровень " + m[2], true
}
