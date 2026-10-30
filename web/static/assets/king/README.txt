Король в Жёлтом: ассеты финального босса (v0.23)
=================================================

Статус: ЧЕРНОВИК. Готовы king_cat_idle, king_cat_talk, king_dark, king_field, king_throne,
знак, музыка и озвучка; остальное заполняется по мере готовности.
Всё, кроме озвучки, генерируется в z-image-turbo (ComfyUI), фон вырезается BiRefNet (rembg),
пиксель-арт уменьшается скриптом и дочищается в Aseprite вручную.

Файлы
-----
файл                  размер        прозр.  откуда                          статус
king_cat_idle.png     587x1020      да      сделан автором (свой арт)       готов
king_cat_talk.png     587x1020      да      сделан автором (пара к idle)    готов
king_cat_grin.png     ~?x1000       да      img2img от idle                 нет
king_cat_point.png    ~?x1000       да      img2img от idle                 нет
king_transform.png    ~?x1000       да      img2img от idle (по желанию)    нет
king_dark.png         698x1020      да      мрачная форма, от автора        готов
king_field.png        40x93         да      пиксель-арт, мрачная форма      готов
faceless.png          32x32         да      генерация + скрипт + Aseprite   нет (по желанию)
king_rules.png        1920x1080     нет     генерация 1536x864 + масштаб    нет (по желанию)
king_throne.png       192x192       да      генерация + скрипт + знак       готов
sign/yellow_sign.jpg  500x600       НЕТ     Жёлтый знак, жёлтый на чёрном   готов
music/lost_carcosa.ogg 3:19 стерео  -       песня, Suno                     готов
voice/*.ogg           18 реплик     -       озвучка, моно 48 кГц            готов (2 нет, см. ниже)

Правила для кода
----------------
- Арты обрезаны по содержимому, низ PNG = подол плаща, якорь ставить внизу по центру.
- king_cat_idle и king_cat_talk одного размера (587x1020) и совпадают ниже головы; отличаются
  только в верхних ~345 px (рот, края капюшона), так что их можно просто менять местами под голос.
- Спрайты поля без полупрозрачности, контур 1 px чёрный, стоят на нижней строке кадра.
- king_field — НЕ квадрат: 40x93, обрезан по содержимому, без полупрозрачности, но ~620 цветов
  (палитра не сведена). Якорь внизу по центру, масштаб целым множителем и nearest.
- king_throne стоит на нижней строке квадратного холста 192x192 (поля по бокам и сверху прозрачные).
  Масштабировать только целым множителем и nearest (scaleMode nearest в PixiJS).
- sign/yellow_sign.jpg — JPEG без альфы: жёлтый знак на чёрном фоне. Накладывать с blendMode
  "add"/"screen" (чёрное станет прозрачным) или сделать из него PNG с альфой.
- Фон king_rules без текста: надпись «Король правит» рисует код. Верхняя треть свободна под неё.

Настройки генерации (ComfyUI, z-image-turbo)
--------------------------------------------
UNETLoader z_image_turbo_bf16, CLIPLoader qwen_3_4b (тип lumina2), VAE ae.safetensors,
ModelSamplingAuraFlow shift 3, KSampler: 8 шагов, CFG 1.0, res_multistep / simple, seed 42 на всю серию.
Негатив пустой (на CFG 1 не работает), всё «без ...» написано в позитиве.

- denoise < 1 ТОЛЬКО вместе с VAE Encode. С EmptySD3LatentImage ставить 1.0,
  иначе получается серый шумный фон и «пережаренная» мозаика (так вышли ComfyUI_00004..06).
- Референс перед VAE Encode — не меньше ~1 Мп (размер выхода = размер входа):
  ref_body_1024x1536.png (кот целиком), ref_head_1024x1024.png (голова).
- Слова helmet / batman / bat дают маску Бэтмена с ушами. Писать «domino eye mask ... no ears, no horns».

Общая часть промпта кота (CAT):
  Semi-realistic digital painting with visible loose brush strokes. A real ginger tabby cat with
  photographic fur and long white whiskers. A simple black domino eye mask made of smooth cloth covers
  only the upper part of its muzzle around the eyes; the mask is flat and has no ears, no horns and no
  bat shapes. The cat wears a mustard-yellow hooded cloak painted with broad soft strokes; the hood fits
  tightly around the face and the cat's own orange ears stick out through the hood. The cloak falls to
  the floor in long smooth folds and hides the body completely; it is one plain fabric with no pattern,
  no gold trim, no blue, no clasp, no strings. Full body, centered, the head takes the upper quarter of
  the picture. Isolated on a plain pure white background, no floor, no shadow, no text, no frame.

king_cat_idle    ref_body, 1024x1536, denoise 0.5
  CAT + The cat faces the viewer and looks straight into the camera, head upright, mouth firmly closed,
  calm, regal and slightly arrogant expression.
king_cat_talk    Load Image = СЫРОЙ idle (до вырезки), в Mask Editor закрасить морду ниже маски,
                 VAE Encode -> SetLatentNoiseMask -> KSampler denoise 0.55, тот же seed
  CAT + The cat faces the viewer, its mouth is half open as if speaking, a pink tongue and two small
  fangs are visible, the rest of the face is unchanged.
king_cat_grin    Load Image = сырой idle, denoise 0.55
  CAT + The head is tilted toward its left shoulder, eyes narrowed, a smug self-satisfied grin with one
  corner of the mouth raised, looking at the viewer from under the mask.
king_cat_point   Load Image = сырой idle, denoise 0.6–0.65 (лапа даётся не с первого раза)
  CAT + One orange furry front paw with pink toe pads comes out from the front opening of the cloak and
  points straight at the viewer, the paw is foreshortened toward the camera, the rest of the body stays
  hidden under the cloak, the cat looks at the viewer with a challenging stare.
king_transform   Load Image = сырой idle, denoise 0.6, фон тёмный (вырезка --bg dark)
  Semi-realistic digital painting with visible loose brush strokes, ominous mood. The same ginger cat in
  the mustard-yellow hooded cloak and black domino mask is changing: the hem of the cloak tears into long
  ragged strips, dark wet black tentacles creep out from under it, a smooth pale ivory mask spreads over
  the left half of the cat's face, the cat's ears stretch taller under the hood, sickly yellow light from
  below. Full body, centered. Isolated on a plain flat dark charcoal gray background, no floor, no fog,
  no text.
king_dark        EmptySD3LatentImage 1024x1536, denoise 1.0 (вырезка --bg dark)
  Dark gothic digital painting with visible loose brush strokes. A very tall thin figure in a tattered
  mustard-yellow robe torn into long ragged strips that hang to the floor. The hood is pulled low, and
  under the hood two pointed cat ears push the fabric up from inside. Instead of a face there is a smooth
  featureless pale ivory mask with no eyes, no mouth and no holes, with one thin crack. Dark wet black
  tentacles coil out from under the torn hem and lie on the ground. No hands are visible. Full body,
  centered, front view, looming over the viewer. Isolated on a plain flat dark charcoal gray background,
  no floor, no fog, no text.
king_field       EmptySD3LatentImage 1024x1024, denoise 1.0, потом: king_tool.py pixel ... --size 48
  Pixel art game sprite in 16-bit style, chunky square pixels, flat colors, limited palette of about
  twelve colors, thick black outline. A small cat king standing: a ginger cat in a mustard-yellow hooded
  cloak that reaches the ground, orange cat ears sticking out of the hood, a black domino mask over the
  eyes. Three-quarter front view, full body, centered, plain pure white background, no shadow, no
  gradients, no text.
faceless         как king_field, потом --size 32
  Pixel art game sprite in 16-bit style, chunky square pixels, flat colors, limited palette of about
  eight colors, thick black outline. A small hunched faceless servant in ragged gray rags with a hood,
  a smooth pale blank mask instead of a face with no eyes and no mouth, thin dark hands hidden in the
  sleeves. Three-quarter front view, full body, centered, plain pure white background, no shadow, no
  gradients, no text.
king_rules       EmptySD3LatentImage 1536x864, denoise 1.0, потом: king_tool.py screen ... (-> 1920x1080)
  Dark cinematic digital painting with visible loose brush strokes. A ruined stone fortress at dusk under
  a sickly yellow sky with two pale suns. In the lower middle a very tall figure in tattered mustard-yellow
  robes with a smooth pale mask sits on a throne made of broken towers, dark tentacles spill down the
  steps, small gray faceless figures kneel around it. The upper third of the picture is empty dark sky.
  Wide shot, no text, no letters, no logo, no frame.
king_throne      Хастур на каменном троне, пиксель-арт 192x192, 32 цвета. Три шага:
  1) EmptySD3LatentImage 1024x1280, denoise 1.0, seed 666 -> img2img (VAE Encode) seed 666, denoise 0.75
     (промпт v2: «no capelet ...» — накидку на плечах НЕ убирает, слова её притягивают).
  2) img2img от результата шага 1, seed 42, denoise 0.7, промпт v3 ниже: накидка ушла, плечи чуть шире.
     При 0.5–0.6 бахрома на груди остаётся, выше 0.7 меняется вся композиция.
  3) king_tool.py pixel ... --size 192 --colors 32, затем sign.py: медальон перерисован (r=10, ~21 px:
     чёрный контур, золотой обод с бликом, тёмная середина) и на него попиксельно положен Жёлтый знак
     11x13 px с картинки автора (нейросеть знак не рисует).
  Промпт v3:
  High quality detailed pixel art in the style of a 32-bit era game, crisp visible square pixels, hard
  edges, no blur, no anti-aliasing, limited palette of about thirty colors, flat shading with clean
  dithering, dark outlines. Hastur, the King in Yellow, a huge broad-shouldered figure with a massive wide
  chest and powerful square shoulders as wide as the back of the throne, sits on a massive throne carved
  from rough gray stone, front view, symmetrical, full figure, the whole throne visible. He wears a simple
  monk's robe of mustard-yellow and ochre cloth: a small tight cowl hood sits close around the head and
  joins the robe right at the neck, the shoulders are smooth and bare of folds, the fabric runs in one
  clean surface from the neck down the shoulders into wide sleeves. The robe is torn and frayed only at
  the hem and the cuffs, falling in heavy folds over his knees and down the steps of the throne. The hood
  is empty: inside it there is no face, no mask, no eyes, only a pitch-black abyss, flat pure black with
  nothing in it. His long bony clawed gray hands rest on the armrests. A large plain round dark medallion
  with a thin gold rim hangs on a chain in the middle of his chest, its face is smooth and empty. Thick
  dark gray-violet tentacles with pale suckers rise from under the robe and coil around the throne, its
  legs and the armrests, curling upward on both sides. Ominous, regal, silent. Isolated on a plain flat
  pure white background, no stars, no sky, no wall behind the throne, no floor, no shadow, no text, no
  letters, no signature, no frame.

Чистка и проверка
-----------------
Скрипт king_tool.py (лежит вне репозитория, в scratchpad сессии ассетов):
  cut   сырой.png king_cat_idle.png --bg white     BiRefNet, снятие белого ореола, обрезка, высота 1000
  cut   сырой.png king_dark.png --bg dark          то же для тёмного фона
  pixel сырой.png king_field.png --size 48         уменьшение (box до 4x, затем nearest), палитра 16,
                                                   альфа 0/255, контур 1 px, превью *_x8.png
  check *.png                                      размер, альфа, углы, обрезка, ореол
  sign.py спрайт.png знак.png out.png cx cy r       перерисовать медальон и положить знак (king_throne: 96 79 10)
  voice voice/*.ogg / vfix in.wav out.ogg          проверка / приведение к моно 48 кГц, -16 LUFS

Звук
----
music/lost_carcosa.ogg  «Lost Carcosa», orangetempo2779, сделано в Suno 2026-10-27 (в тегах файла).
                        Vorbis 48 кГц стерео, 3:19, -18.6 LUFS — для фоновой музыки нормально
                        (громкость регулирует код). Права на использование в стриме зависят от
                        тарифа Suno на момент создания трека.
voice/*.ogg             Все 18 файлов: Vorbis моно 48 кГц, -16 LUFS (±0,5), true peak не выше -1,5 dBFS,
                        тишина по краям 0,1 с. 2026-10-28 все пересобраны из mp4 с эхом (voice_fx.py
                        в scratchpad сессии ассетов): без понижения голоса, без хрипа, гула и т. п.
  Эхо:                  короткое (30/55 мс) — реплики кота в сцене; среднее (60/110/170 мс) —
                        king_transform и реплики в бою; длинное (120–600 мс) — только king_dark,
                        последняя реплика сцены перед боем.
  По списку дизайна:    king_hello_few 5.0 с, king_hello_many 5.5, king_towers_few 4.1,
                        king_towers_many 3.8, king_top 6.2, king_waves 3.1, king_drunk 3.2,
                        king_transform 4.7, king_dark 4.0, king_phase2 1.8, king_song 3.7,
                        king_mask 2.9, king_name 1.5, king_sign_lost 4.5, king_rules 5.7.
  Сверх списка:         king_bob 6.0 с, king_dance 7.2, king_repair 5.0.
  НЕТ:                  king_towers_some, king_fall (для них в voice_fx.py уже задано эхо:
                        короткое и среднее — положить mp4 и перезапустить скрипт).
  Прежние версии:       все 18 без эха сохранены в scratchpad (voice_backup_full).
voice/*.mp4             Исходники озвучки (видео 1920x1080 + AAC), 18 файлов, ~138 МБ. Игре не нужны:
                        их лучше убрать из web/static до коммита, иначе они попадут в репозиторий.

Вручную
-------
king_cat_idle.png, king_cat_talk.png — сделаны автором, не через ComfyUI (промпты выше для них не
использовались). Проверены king_tool.py check: альфа есть, обрезаны по содержимому, без ореола.
king_throne.png — медальон и Жёлтый знак положены скриптом, не нейросетью; в Aseprite не правился.
king_dark.png, king_field.png, sign/, music/, voice/ — добавлены автором вне этой сессии; здесь только
проверены (размеры, альфа, громкость). Как сделаны king_dark и king_field, не записано.
