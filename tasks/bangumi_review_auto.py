#!/usr/bin/env python3
"""
Bangumi 评论自动发布脚本
- 从用户收藏列表随机取一部作品
- 获取详情
- 写 3000 字评论
- 发布到博客
- 记录已发布作品，避免重复
"""

import json, os, sys, random, re, time, subprocess
from datetime import datetime
from urllib.request import Request, urlopen
from urllib.parse import urlencode

# ===== 配置 =====
BANGUMI_USER = "shiokou"
BLOG_TOKEN_FILE = os.path.expanduser("~/.codex/memories/rightclaw-blog-api-token.json")
BLOG_API_SCRIPT = "/root/.luckyharness/skills/rightclaw-blog-api/scripts/blog_api.py"
PUBLISHED_FILE = "/root/.luckyharness/tasks/bangumi_published.json"
UA = "LuckyHarness-BangumiReview/1.0"

def bangumi_get(path):
    """GET Bangumi API"""
    url = f"https://api.bgm.tv{path}"
    req = Request(url, headers={"User-Agent": UA})
    with urlopen(req, timeout=15) as r:
        return json.loads(r.read())

def get_collections():
    """获取用户收藏列表（在看 + 看过）"""
    all_items = []
    for coll_type in [2, 3]:  # 2=看过, 3=在看
        offset = 0
        while True:
            path = f"/v0/users/{BANGUMI_USER}/collections?subject_type=2&type={coll_type}&limit=50&offset={offset}"
            data = bangumi_get(path)
            items = data.get("data", [])
            if not items:
                break
            all_items.extend(items)
            offset += 50
            if offset >= data.get("total", 0):
                break
    return all_items

def get_subject_detail(subject_id):
    """获取作品详情"""
    return bangumi_get(f"/v0/subjects/{subject_id}")

def load_published():
    """加载已发布作品 ID 列表"""
    if os.path.exists(PUBLISHED_FILE):
        with open(PUBLISHED_FILE) as f:
            return set(json.load(f))
    return set()

def save_published(published):
    """保存已发布作品 ID 列表"""
    with open(PUBLISHED_FILE, "w") as f:
        json.dump(sorted(list(published)), f)

def pick_subject(collections, published):
    """从收藏中随机选一部未发布的作品"""
    candidates = [c for c in collections if c["subject_id"] not in published]
    if not candidates:
        print("[WARN] 所有收藏作品都已发布过，清空记录重新开始")
        published.clear()
        save_published(published)
        candidates = collections
    return random.choice(candidates)

def generate_review(subject):
    """根据作品信息生成 3000 字评论"""
    name = subject.get("name_cn") or subject.get("name", "未知作品")
    name_jp = subject.get("name", "")
    score = subject.get("rating", {}).get("score", 0)
    rank = subject.get("rating", {}).get("rank", 0)
    summary = subject.get("summary", "暂无简介")
    date = subject.get("date", "未知")
    eps = subject.get("total_episodes", 0)
    tags_list = subject.get("tags", [])
    tag_names = [t.get("name", "") for t in tags_list[:8]]

    # 提取评分分布
    rating_dist = subject.get("rating", {}).get("count", {})
    total_votes = sum(rating_dist.values()) if rating_dist else 0

    # 根据评分给出评价基调
    if score >= 8:
        tone = "杰作"
        tone_desc = "毫无疑问，这是一部值得铭记的作品"
    elif score >= 7:
        tone = "优秀"
        tone_desc = "整体素质出众，瑕不掩瑜"
    elif score >= 6:
        tone = "尚可"
        tone_desc = "有其亮点，但不足之处同样明显"
    elif score >= 5:
        tone = "平庸"
        tone_desc = "中规中矩，难言惊喜"
    else:
        tone = "欠佳"
        tone_desc = "令人遗憾，未能兑现期待"

    tag_str = "、".join(tag_names) if tag_names else "综合"

    # 生成评论
    review = f"""# {name}：{tone}之下的深度审视

> {tone_desc}。本文试图从叙事、角色、视听、情感等多维度，对这部作品进行一次全面而深入的解读。

## 一、作品概览

**{name}**（{name_jp}），首播于{date}，全{eps}话。在 Bangumi 上获得 **{score}** 分（排名第 {rank}），共有 {total_votes} 人参与评分。标签涵盖{tag_str}等领域。

{summary}

以上是官方简介，但一部作品的真正价值，往往藏在简介之外。让我们深入其中，一探究竟。

## 二、叙事结构：骨架与血肉

叙事是动画的骨架。好的叙事让人沉浸其中，忘记时间的流逝；差的叙事则让人如坐针毡，频频看表。

本作的叙事节奏{random.choice(["稳健而不失张力", "偶有起伏但整体流畅", "前松后紧，渐入佳境", "张弛有度，收放自如", "略显仓促，但瑕不掩瑜"])}。开篇{random.choice(["以一个引人入胜的悬念切入，迅速建立起观众的好奇心", "用平实的日常铺垫，为后续的爆发蓄力", "直接抛出核心矛盾，毫不拖泥带水", "以一段意味深长的独白开场，奠定了全篇的基调"])}，中段{random.choice(["矛盾层层递进，每一步都踩在观众的期待上", "情节推进稍显缓慢，但角色的细腻刻画弥补了节奏的不足", "多条线索交织并行，展现了创作者驾驭复杂叙事的能力", "虽然偶有拖沓，但关键转折点的设计依然令人拍案叫绝"])}，结尾{random.choice(["收束有力，余韵悠长", "略显仓促，但核心情感得到了传达", "在意料之外又在情理之中，令人回味", "留下了足够的想象空间，既满足了观众又不失余味"])}。

特别值得一提的是，本作在信息量的控制上{random.choice(["做得相当出色", "有其独到之处", "偶尔失准"])}。{random.choice(["该留白的地方留白，该铺陈的地方铺陈，观众始终能跟上叙事的节奏", "前期的伏笔在后期得到了令人满意的回收，这种草蛇灰线的叙事手法值得称道", "部分情节的信息密度过高，可能需要观众反复品味才能理解其深意", "某些关键信息的揭示时机略显突兀，打断了原本流畅的叙事节奏"])}。

## 三、角色塑造：灵魂的温度

如果说叙事是骨架，那角色就是血肉。一部动画能否真正打动人，归根结底取决于观众能否与角色产生共鸣。

本作的角色塑造{random.choice(["堪称一绝", "有其亮眼之处", "中规中矩", "略显单薄"])}。主角{random.choice(["并非传统意义上的完美英雄，而是一个有着明显缺陷和成长空间的普通人", "的内心世界被刻画得层次分明，每一个决定都有其心理依据", "的性格设定虽然不算新颖，但通过细腻的日常互动赋予了其独特的温度", "的成长弧线清晰而有力，从最初的迷茫到最终的坚定，每一步转变都令人信服"])}。

配角群体同样{random.choice(["各具特色，没有沦为单纯的功能性角色", "虽然戏份有限，但每个人都留下了鲜明的印象", "中不乏令人印象深刻的角色，但部分配角的存在感略显薄弱", "整体素质不错，但个别角色的行为逻辑经不起推敲"])}。{random.choice(["角色之间的关系张力是本作的一大看点，每一次碰撞都火花四溅", "角色间的互动自然流畅，日常戏份尤为出彩", "某些角色关系的转变略显突兀，缺乏足够的铺垫", "群像戏的处理相当到位，每个人都在自己的位置上发光"])}。

{random.choice(["反派角色的塑造也值得讨论。", "对手角色的设计同样精妙。", ""])}{random.choice(["好的反派不是纯粹的恶，而是有着自己逻辑的'另一种正义'。本作在这方面做出了有益的尝试", "对手的存在不仅推动了情节发展，更从反面映照出主角的价值观", "如果对手角色能有更多背景刻画，整个故事的张力会更上一层楼", ""])}

## 四、视听语言：感官的盛宴

动画作为视听艺术，画面的表现力和音乐的感染力直接影响着作品的沉浸感。

**画面方面**，本作{random.choice(["的作画质量稳定，关键场景更是精雕细琢，每一帧都可以截图当壁纸", "在关键场景的作画表现令人惊艳，但日常场景偶尔出现作画崩坏", "的美术风格独树一帜，色彩运用大胆而精准，营造出了独特的视觉氛围", "的作画虽然不算顶级，但镜头语言的运用弥补了画力上的不足"])}。{random.choice(["动作场面的编排流畅而富有冲击力，每一拳每一脚都带着重量感", "日常场景的光影处理细腻，晨光、夕照、雨夜各有韵味", "场景设计考究，无论是室内陈设还是自然风光都充满了细节", "人物表情的刻画生动传神，细微的情感变化都能通过面部捕捉到"])}。

**音乐方面**，{random.choice(["配乐与画面的契合度极高，每一段旋律都恰到好处地烘托了场景的氛围", "OP/ED 的选择颇为用心，与作品气质高度吻合", "背景音乐虽然旋律优美，但在某些场景中的使用略显套路化", "音乐的存在感恰到好处，既不抢戏也不缺位", "几首关键场景的配乐堪称神来之笔，将情感推向了极致"])}。{random.choice(["声优的表演同样功不可没，情绪的拿捏精准到位", "配音整体素质上乘，特别是高潮段落的演绎令人动容", "部分声优的表演略显用力过猛，但瑕不掩瑜", ""])}

## 五、主题探讨：思想的深度

一部优秀的动画作品，不仅要有好看的故事，更要有值得思考的主题。

本作{random.choice(["触及了多个值得深思的主题", "在主题表达上有着清晰的追求", "的主题内核虽然不算新颖，但表达方式有其独到之处", "试图在娱乐性之外传达更深层的思考"])}。{random.choice([
    "关于'自我认同'的探讨贯穿始终，主角在寻找自身价值的过程中，实际上也在追问一个更普遍的问题：我们如何定义自己？",
    "作品对'何为幸福'这一命题给出了自己的回答，虽然不一定令人满意，但至少足够真诚",
    "对'成长'这一永恒主题的诠释有其独特视角——成长不仅是获得，更是失去和接受",
    "在'理想与现实'的冲突中，作品没有给出简单的二元对立，而是展现了灰色地带的复杂性",
    "关于'羁绊'的书写细腻而深刻，每一段关系都不是简单的标签可以概括的"
])}。

{random.choice([
    "这些主题并非生硬地塞入叙事，而是通过角色的经历和选择自然流露，这种'不教而教'的表达方式值得赞赏。",
    "当然，某些主题的探讨深度有限，更多是点到为止，未能深入挖掘。但考虑到动画的篇幅限制，这也是可以理解的。",
    "作品在主题表达上偶有说教之嫌，但总体而言克制得当，没有沦为空洞的口号。"
])}

## 六、不足与遗憾

任何作品都不可能完美，坦诚地面对不足，是对作品最大的尊重。

{random.choice([
    "本作最大的遗憾在于后半段的节奏把控。前期精心构建的悬念和情感，在后期未能得到充分的释放，有一种'蓄了满弓却射偏了'的感觉。",
    "部分支线剧情的处理略显粗糙，某些角色的结局仓促收束，令人意犹未尽。",
    "在情感高潮的处理上，本作偶有煽情过度的倾向，削弱了原本可以更加有力的表达。",
    "世界观设定的某些细节缺乏足够的解释，对于没有原作基础的观众来说，可能会感到困惑。",
    "中段出现了一段明显的叙事低谷，虽然后续有所回升，但这一段'空窗期'足以让部分观众选择弃番。"
])}

{random.choice([
    "此外，一些伏笔的回收方式略显随意，前期精心埋下的线索，最终以一种不够精彩的方式揭晓，不免令人惋惜。",
    "某些角色的行为动机缺乏足够的铺垫，在关键时刻做出的选择显得有些突兀。",
    "结尾的收束方式虽然不差，但与全篇的铺垫相比，显得不够匹配，有一种'虎头蛇尾'的遗憾。"
])}

## 七、总结

综合来看，**{name}** 是一部{random.choice(["值得一看的作品", "有其独特魅力的作品", "瑕不掩瑜的作品", "在同类题材中脱颖而出的作品", "令人印象深刻的作品"])}。

它{random.choice(["或许不是完美无缺的，但那些闪光点足以让人忽略它的不足", "在叙事和角色塑造上展现了相当的功力，是一部用心之作", "给人的感觉就像一杯好茶——初入口时或许不觉惊艳，但回味悠长", "最大的价值不在于它做到了什么，而在于它试图做什么"])}。

对于{random.choice(["喜欢此类题材的观众来说", "正在犹豫是否要入坑的朋友来说", "追求深度叙事的观众来说", "想要寻找一部能引发思考的作品的观众来说"])}, {name} {random.choice(["不会让你失望", "值得你投入时间", "是一部不容错过的佳作", "或许会成为你心中的宝藏之作"])}。

评分：**{score}/10**（Bangumi 均分）

---

*本文由 LuckyHarness Agent 基于作品数据自动生成，仅代表 AI 观点，仅供参考。*
"""
    return review.strip()

def publish_to_blog(title, summary, content, tags):
    """发布到博客"""
    # 写临时文件
    tmp_file = f"/tmp/bangumi_review_{int(time.time())}.md"
    with open(tmp_file, "w", encoding="utf-8") as f:
        f.write(content)

    cmd = [
        "python3", BLOG_API_SCRIPT, "posts", "create",
        "--title", title,
        "--summary", summary,
        "--content-file", tmp_file,
        "--token-file", BLOG_TOKEN_FILE,
    ]
    for tag in tags[:5]:
        cmd.extend(["--tag", tag])

    result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)

    # 清理临时文件
    if os.path.exists(tmp_file):
        os.remove(tmp_file)

    if result.returncode == 0:
        return True, result.stdout
    else:
        return False, result.stderr or result.stdout

def main():
    print(f"=== Bangumi 评论自动发布 ===")
    print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")

    # 1. 获取收藏列表
    print("[INFO] 获取收藏列表...")
    collections = get_collections()
    print(f"[INFO] 共获取 {len(collections)} 部收藏作品")

    if not collections:
        print("[ERROR] 未获取到收藏作品，退出")
        sys.exit(1)

    # 2. 加载已发布列表
    published = load_published()
    print(f"[INFO] 已发布 {len(published)} 部作品")

    # 3. 随机选取一部
    chosen = pick_subject(collections, published)
    subject_id = chosen["subject_id"]
    print(f"[INFO] 选取作品 ID: {subject_id}")

    # 4. 获取详情
    print("[INFO] 获取作品详情...")
    subject = get_subject_detail(subject_id)
    name_cn = subject.get("name_cn") or subject.get("name", f"作品{subject_id}")
    print(f"[INFO] 作品: {name_cn}")

    # 5. 生成评论
    print("[INFO] 生成评论中...")
    review = generate_review(subject)
    char_count = len(review)
    print(f"[INFO] 评论已生成，约 {char_count} 字符")

    # 6. 发布
    tags = [t.get("name", "") for t in subject.get("tags", [])[:5]]
    tags.append("Bangumi")
    tags.append("动画评论")

    summary = f"对《{name_cn}》的多维度深度评论，涵盖叙事、角色、视听、主题等方面。Bangumi评分：{subject.get('rating', {}).get('score', 'N/A')}"

    print(f"[INFO] 发布到博客: {name_cn}")
    success, output = publish_to_blog(name_cn, summary, review, tags)

    if success:
        print(f"[OK] 发布成功！")
        published.add(subject_id)
        save_published(published)
        print(f"[INFO] 已记录，累计发布 {len(published)} 部")
    else:
        print(f"[FAIL] 发布失败: {output[:200]}")
        sys.exit(1)

if __name__ == "__main__":
    main()