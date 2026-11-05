package seed

// Demo catalog for a Nicaraguan liquor store. Prices are córdobas before VAT for the
// 750 ml (or base) size; other sizes scale from it.

type size struct {
	label string
	ml    int
	scale float64
}

var (
	s375  = size{"375 ml", 375, 0.56}
	s750  = size{"750 ml", 750, 1}
	s1000 = size{"1 L", 1000, 1.28}
	s1750 = size{"1.75 L", 1750, 2.05}
)

type line struct {
	category string
	brand    string
	name     string
	alcohol  string
	price    float64
	sizes    []size
	unit     string
	// pack adds six-pack and 24-can box presentations (beers).
	pack bool
}

func spirits(category, brand, alcohol string, items ...struct {
	name  string
	price float64
	sizes []size
}) []line {
	out := make([]line, 0, len(items))
	for _, it := range items {
		out = append(out, line{category: category, brand: brand, name: it.name, alcohol: alcohol, price: it.price, sizes: it.sizes, unit: "bot"})
	}
	return out
}

type item = struct {
	name  string
	price float64
	sizes []size
}

var all4 = []size{s375, s750, s1000, s1750}

func demoLines() []line {
	var lines []line
	add := func(l ...line) { lines = append(lines, l...) }

	add(spirits("Ron", "Flor de Caña", "35",
		item{"Flor de Caña 4 años Extra Lite", 218.5, all4},
		item{"Flor de Caña 4 años Extra Seco", 224, all4},
		item{"Flor de Caña 5 años Añejo Clásico", 289, []size{s375, s750, s1000}},
		item{"Flor de Caña 7 años Gran Reserva", 421.75, all4},
		item{"Flor de Caña 12 años Centenario", 856, []size{s750, s1000}},
		item{"Flor de Caña 18 años Centenario", 1647.8, []size{s750}},
		item{"Flor de Caña 25 años Centenario", 3980, []size{s750}},
		item{"Flor de Caña Spresso", 512, []size{s750}},
	)...)
	add(spirits("Ron", "Ron Plata", "35",
		item{"Ron Plata Blanco", 140, all4},
		item{"Ron Plata Oro", 152, []size{s750, s1000}},
	)...)
	add(spirits("Ron", "Bacardí", "40", item{"Bacardí Carta Blanca", 468, []size{s750, s1000}}, item{"Bacardí Añejo Cuatro", 690, []size{s750}})...)
	add(spirits("Ron", "Havana Club", "40", item{"Havana Club Añejo 3 años", 520, []size{s750}})...)
	add(spirits("Ron", "Zacapa", "40", item{"Ron Zacapa 23", 2450, []size{s750}})...)
	add(spirits("Ron", "Diplomático", "40", item{"Diplomático Reserva Exclusiva", 1890, []size{s750}})...)
	add(spirits("Ron", "Captain Morgan", "35", item{"Captain Morgan Spiced", 598, []size{s750, s1000}})...)
	add(spirits("Ron", "Botran", "40",
		item{"Botran Añejo 8 años", 640, []size{s750}},
		item{"Botran Reserva 12 años", 890, []size{s750}},
		item{"Botran Reserva Blanca", 590, []size{s750}},
		item{"Botran Solera 1893", 1790, []size{s750}},
	)...)
	add(spirits("Ron", "Centenario", "40", item{"Ron Centenario 7 años", 720, []size{s750}}, item{"Ron Centenario 12 años", 1120, []size{s750}})...)
	add(spirits("Whisky", "Dewar's", "40", item{"Dewar's White Label", 690, []size{s750, s1000}})...)
	add(spirits("Whisky", "Ballantine's", "40", item{"Ballantine's Finest", 720, []size{s750, s1000}})...)
	add(spirits("Whisky", "Glenfiddich", "40", item{"Glenfiddich 12 años", 2290, []size{s750}})...)
	add(spirits("Whisky", "The Macallan", "40", item{"The Macallan 12 años Double Cask", 4650, []size{s750}})...)

	add(spirits("Whisky", "Johnnie Walker", "40",
		item{"Johnnie Walker Red Label", 782.6, []size{s375, s750, s1000}},
		item{"Johnnie Walker Black Label 12 años", 1452.2, []size{s375, s750, s1000}},
		item{"Johnnie Walker Double Black", 1890, []size{s750}},
		item{"Johnnie Walker Gold Reserve", 2780, []size{s750}},
	)...)
	add(spirits("Whisky", "Buchanan's", "40",
		item{"Buchanan's Deluxe 12 años", 1496.5, []size{s375, s750, s1000}},
		item{"Buchanan's 18 años Special Reserve", 3420, []size{s750}},
		item{"Buchanan's Master", 1990, []size{s750}},
	)...)
	add(spirits("Whisky", "Old Parr", "40", item{"Old Parr 12 años", 1348.7, []size{s750, s1000}})...)
	add(spirits("Whisky", "Chivas Regal", "40", item{"Chivas Regal 12 años", 1420, []size{s750, s1000}}, item{"Chivas Regal 18 años", 3650, []size{s750}})...)
	add(spirits("Whisky", "Jack Daniel's", "40", item{"Jack Daniel's Old No. 7", 1150, []size{s375, s750, s1000}}, item{"Jack Daniel's Honey", 1210, []size{s750}})...)
	add(spirits("Whisky", "Jameson", "40", item{"Jameson Irish Whiskey", 990, []size{s750, s1000}})...)
	add(spirits("Whisky", "Grant's", "40", item{"Grant's Triple Wood", 640, []size{s750, s1000}})...)
	add(spirits("Whisky", "Something Special", "40", item{"Something Special", 870, []size{s750}})...)

	add(spirits("Vodka", "Absolut", "40", item{"Absolut Vodka", 716.5, []size{s375, s750, s1000}}, item{"Absolut Citron", 760, []size{s750}})...)
	add(spirits("Vodka", "Smirnoff", "37.5", item{"Smirnoff No. 21", 428.7, all4}, item{"Smirnoff Ice", 52, []size{{"355 ml", 355, 1}}})...)
	add(spirits("Vodka", "Grey Goose", "40", item{"Grey Goose", 1690, []size{s750}})...)
	add(spirits("Vodka", "Stolichnaya", "40", item{"Stolichnaya", 690, []size{s750}})...)
	add(spirits("Vodka", "Ketel One", "40", item{"Ketel One", 980, []size{s750}})...)

	add(spirits("Ginebra", "Tanqueray", "43.1", item{"Tanqueray London Dry", 893.9, []size{s750, s1000}})...)
	add(spirits("Ginebra", "Bombay", "40", item{"Bombay Sapphire", 990, []size{s750, s1000}})...)
	add(spirits("Ginebra", "Beefeater", "40", item{"Beefeater London Dry", 760, []size{s750}})...)
	add(spirits("Ginebra", "Gordon's", "37.5", item{"Gordon's London Dry", 520, []size{s750}})...)
	add(spirits("Ginebra", "Hendrick's", "41.4", item{"Hendrick's", 1720, []size{s750}})...)

	add(spirits("Tequila", "José Cuervo", "38", item{"José Cuervo Especial Reposado", 690, []size{s375, s750, s1000}}, item{"José Cuervo Tradicional Plata", 820, []size{s750}})...)
	add(spirits("Tequila", "Don Julio", "38", item{"Don Julio Blanco", 1890, []size{s750}}, item{"Don Julio Reposado", 2150, []size{s750}})...)
	add(spirits("Tequila", "Patrón", "40", item{"Patrón Silver", 2290, []size{s750}})...)
	add(spirits("Tequila", "1800", "38", item{"1800 Reposado", 1350, []size{s750}})...)
	add(spirits("Tequila", "Olmeca", "35", item{"Olmeca Blanco", 590, []size{s750, s1000}})...)

	add(spirits("Licores", "Baileys", "17", item{"Baileys Original Irish Cream", 790, []size{s375, s750}})...)
	add(spirits("Licores", "Kahlúa", "20", item{"Kahlúa Licor de Café", 690, []size{s750}})...)
	add(spirits("Licores", "Jägermeister", "35", item{"Jägermeister", 980, []size{s750, s1000}})...)
	add(spirits("Licores", "Disaronno", "28", item{"Amaretto Disaronno", 1050, []size{s750}})...)
	add(spirits("Licores", "Licor 43", "31", item{"Licor 43 Original", 1180, []size{s750}})...)
	add(spirits("Licores", "Cointreau", "40", item{"Cointreau", 1290, []size{s750}})...)
	add(spirits("Licores", "Campari", "25", item{"Campari Bitter", 820, []size{s750}})...)
	add(spirits("Licores", "Aperol", "11", item{"Aperol Aperitivo", 760, []size{s750}})...)
	add(spirits("Licores", "Malibu", "21", item{"Malibu Coco", 620, []size{s750}})...)
	add(spirits("Licores", "Bols", "15", item{"Bols Triple Sec", 410, []size{s750}})...)

	wines := []struct {
		brand, name string
		price       float64
	}{
		{"Concha y Toro", "Casillero del Diablo Cabernet Sauvignon", 478.25},
		{"Concha y Toro", "Casillero del Diablo Merlot", 478.25},
		{"Concha y Toro", "Casillero del Diablo Carmenere", 478.25},
		{"Concha y Toro", "Casillero del Diablo Sauvignon Blanc", 468},
		{"Concha y Toro", "Frontera Cabernet Sauvignon", 248},
		{"Concha y Toro", "Frontera Merlot", 248},
		{"Concha y Toro", "Reservado Cabernet Sauvignon", 289},
		{"San Pedro", "Gato Negro Cabernet Sauvignon", 261.7},
		{"San Pedro", "Gato Negro Merlot", 261.7},
		{"San Pedro", "Gato Negro Chardonnay", 261.7},
		{"Santa Rita", "Santa Rita 120 Cabernet Sauvignon", 318},
		{"Santa Rita", "Santa Rita 120 Merlot", 318},
		{"Trapiche", "Trapiche Malbec", 356},
		{"Trapiche", "Trapiche Cabernet Sauvignon", 356},
		{"Undurraga", "Undurraga Aliwen Cabernet", 298},
		{"Yellow Tail", "Yellow Tail Shiraz", 412},
		{"Riunite", "Riunite Lambrusco", 290},
		{"Freixenet", "Freixenet Cordón Negro Brut", 590},
		{"Martini", "Martini Asti Spumante", 640},
		{"Marqués de Riscal", "Marqués de Riscal Rioja Reserva", 980},
		{"Catena", "Catena Malbec", 890},
		{"Luigi Bosca", "Luigi Bosca Malbec", 760},
		{"Santa Carolina", "Santa Carolina Reservado Cabernet", 268},
		{"Santa Carolina", "Santa Carolina Reservado Merlot", 268},
		{"Barefoot", "Barefoot Moscato", 420},
		{"Barefoot", "Barefoot Pink Moscato", 420},
		{"Sangre de Toro", "Sangre de Toro Tinto", 452},
		{"Cono Sur", "Cono Sur Bicicleta Pinot Noir", 386},
	}
	for _, w := range wines {
		add(line{category: "Vino", brand: w.brand, name: w.name, alcohol: "13", price: w.price, sizes: []size{s750}, unit: "bot"})
	}

	beers := []struct {
		brand, name, alcohol string
		price                float64
	}{
		{"Toña", "Toña", "4.6", 31.3},
		{"Toña", "Toña Light", "4.0", 31.3},
		{"Victoria", "Victoria Clásica", "4.6", 31.3},
		{"Victoria", "Victoria Frost", "4.7", 33.9},
		{"Heineken", "Heineken", "5.0", 47.85},
		{"Corona", "Corona Extra", "4.5", 46},
		{"Stella Artois", "Stella Artois", "5.0", 52},
		{"Modelo", "Modelo Especial", "4.5", 45},
		{"Bud Light", "Bud Light", "4.2", 38},
		{"Miller", "Miller Lite", "4.2", 38},
		{"Sol", "Sol", "4.5", 40},
		{"Michelob", "Michelob Ultra", "4.2", 44},
	}
	for _, b := range beers {
		add(line{category: "Cerveza", brand: b.brand, name: b.name + " lata", alcohol: b.alcohol, price: b.price, sizes: []size{{"355 ml", 355, 1}}, unit: "lata", pack: true})
		add(line{category: "Cerveza", brand: b.brand, name: b.name + " botella", alcohol: b.alcohol, price: b.price + 3, sizes: []size{{"355 ml", 355, 1}}, unit: "bot", pack: true})
	}
	add(line{category: "Cerveza", brand: "Toña", name: "Toña litro", alcohol: "4.6", price: 68, sizes: []size{{"1 L", 1000, 1}}, unit: "bot"})
	add(line{category: "Cerveza", brand: "Victoria", name: "Victoria Clásica litro", alcohol: "4.6", price: 68, sizes: []size{{"1 L", 1000, 1}}, unit: "bot"})

	mixers := []struct {
		brand, name, size, unit string
		ml                      int
		price                   float64
	}{
		{"Coca-Cola", "Coca-Cola", "355 ml", "lata", 355, 21},
		{"Coca-Cola", "Coca-Cola", "600 ml", "bot", 600, 26},
		{"Coca-Cola", "Coca-Cola", "1.5 L", "bot", 1500, 48},
		{"Coca-Cola", "Coca-Cola", "2 L", "bot", 2000, 64.35},
		{"Coca-Cola", "Coca-Cola", "3 L", "bot", 3000, 86},
		{"Coca-Cola", "Coca-Cola Sin Azúcar", "600 ml", "bot", 600, 26},
		{"Coca-Cola", "Coca-Cola Sin Azúcar", "2 L", "bot", 2000, 64.35},
		{"Coca-Cola", "Sprite", "600 ml", "bot", 600, 24},
		{"Coca-Cola", "Sprite", "2 L", "bot", 2000, 58},
		{"Coca-Cola", "Fanta Naranja", "2 L", "bot", 2000, 58},
		{"Canada Dry", "Canada Dry Ginger Ale", "355 ml", "lata", 355, 24},
		{"Canada Dry", "Agua Tónica Canada Dry", "355 ml", "lata", 355, 24},
		{"Schweppes", "Schweppes Agua Tónica", "355 ml", "lata", 355, 26},
		{"Fuente Pura", "Agua Fuente Pura", "600 ml", "bot", 600, 17.4},
		{"Fuente Pura", "Agua Fuente Pura", "1 L", "bot", 1000, 24},
		{"Fuente Pura", "Agua Fuente Pura", "1.5 L", "bot", 1500, 30},
		{"Perrier", "Agua mineral Perrier", "330 ml", "bot", 330, 48},
		{"Red Bull", "Red Bull Energy Drink", "250 ml", "lata", 250, 52},
		{"Monster", "Monster Energy", "473 ml", "lata", 473, 58},
		{"Del Valle", "Jugo Del Valle Naranja", "1 L", "bot", 1000, 52},
		{"Del Valle", "Jugo Del Valle Durazno", "1 L", "bot", 1000, 52},
		{"Hielo Polar", "Hielo en bolsa", "5 lb", "bolsa", 0, 39.15},
		{"Hielo Polar", "Hielo en bolsa", "10 lb", "bolsa", 0, 68},
		{"Clamato", "Clamato Original", "946 ml", "bot", 946, 98},
		{"Squirt", "Squirt Toronja", "2 L", "bot", 2000, 56},
	}
	for _, m := range mixers {
		add(line{category: "Mezcladores", brand: m.brand, name: m.name, price: m.price, sizes: []size{{m.size, m.ml, 1}}, unit: m.unit})
	}

	snacks := []struct {
		brand, name, size string
		price             float64
	}{
		{"Pringles", "Pringles Original", "124 g", 98},
		{"Pringles", "Pringles Crema y Cebolla", "124 g", 98},
		{"Lay's", "Lay's Clásicas", "150 g", 72},
		{"Doritos", "Doritos Nacho", "150 g", 72},
		{"Tortillitas", "Tajadas de plátano verde", "120 g", 35},
		{"Kiwi", "Maní salado", "200 g", 46},
		{"Kiwi", "Maní japonés", "200 g", 52},
		{"Desechables", "Vasos plásticos 16 oz", "50 u", 85},
		{"Desechables", "Servilletas", "100 u", 32},
		{"Limón", "Limones", "bolsa 10 u", 25},
	}
	for _, sn := range snacks {
		add(line{category: "Snacks y desechables", brand: sn.brand, name: sn.name, price: sn.price, sizes: []size{{sn.size, 0, 1}}, unit: "paq"})
	}
	return lines
}

var demoCustomers = []struct{ name, taxID, phone, email string }{
	{"Rosa Amelia Guevara", "001-140387-0012K", "8834 1029", "rosa.guevara@correo.ni"},
	{"Distribuidora Los Robles S.A.", "J0310000214587", "2270 4418", "compras@losrobles.com.ni"},
	{"Ernesto Lacayo Pallais", "001-250779-0031B", "8471 6650", ""},
	{"Bar y Restaurante La Ceiba", "J0310000391022", "2252 9103", "administracion@laceiba.ni"},
	{"Karla Vanessa Mairena", "281-061192-0004P", "5728 3341", "karla.mairena@correo.ni"},
	{"Hotel Casa Colonial", "J0310000507741", "2552 1180", "eventos@casacolonial.ni"},
	{"Julio César Obando", "001-030585-0049F", "8902 7716", ""},
	{"Eventos y Banquetes Masaya", "J0310000622019", "2522 4407", "ventas@banquetesmasaya.ni"},
}

var demoSuppliers = []struct {
	name, taxID, contact, phone, email string
	credit                             int
}{
	{"Compañía Licorera de Nicaragua", "J0310000000116", "Mario Castillo", "2266 1234", "ventas@licorera.com.ni", 30},
	{"Compañía Cervecera de Nicaragua", "J0310000000221", "Ana Lucía Vega", "2255 7700", "pedidos@ccn.com.ni", 15},
	{"Casa Pellas Importaciones", "J0310000018832", "Roberto Pérez", "2274 4400", "importaciones@pellas.ni", 30},
	{"Distribuidora Vinos del Sur", "J0310000452209", "Claudia Mena", "2278 9015", "info@vinosdelsur.ni", 45},
	{"Embotelladora Nacional (Coca-Cola)", "J0310000001190", "Henry Duarte", "2251 3000", "clientes@enal.com.ni", 7},
	{"Hielo Polar S.A.", "J0310000377104", "Francisco Ruiz", "2249 6060", "pedidos@hielopolar.ni", 0},
}
