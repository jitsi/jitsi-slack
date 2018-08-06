package jitsi

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

var (
	nouns = []string{
		"Aliens", "Animals", "Antelopes", "Ants", "Apes", "Apples", "Baboons",
		"Cars", "Badgers", "Bananas", "Bats", "Bears", "Birds", "Bonobos",
		"Brides", "Bugs", "Bulls", "Butterflies", "Cheetahs", "Cherries", "Chicken",
		"Pears", "Chimps", "Days", "Cows", "Creatures", "Dinosaurs", "Dogs",
		"Dolphins", "Donkeys", "Dragons", "Ducks", "Sounds", "Eagles", "Elephants",
		"Elves", "Fathers", "Fish", "Flowers", "Frogs", "Fruit", "Fungi",
		"Galaxies", "Geese", "Goats", "Gorillas", "Hedgehogs", "Hippos", "Horses",
		"Hunters", "Insects", "Avocados", "Books", "Lemons", "Lemurs", "Leopards",
		"LifeForms", "Lions", "Lizards", "Mice", "Monkeys", "Monsters", "Mushrooms",
		"Crowns", "Oranges", "Orangutans", "Organisms", "Pants", "Parrots",
		"Penguins", "People", "Pigeons", "Pigs", "Pineapples", "Plants", "Potatoes",
		"Priests", "Rats", "Reptiles", "Reptilians", "Rhinos", "Seagulls", "Sheep",
		"Siblings", "Snakes", "Spaghetti", "Spiders", "Squid", "Squirrels",
		"Stars", "Students", "Teachers", "Tigers", "Tomatoes", "Trees", "Vampires",
		"Vegetables", "Bison", "Vulcans", "Weasels", "Werewolves", "Whales",
		"Headphones", "Wizards", "Wolves", "Workers", "Worms", "Zebras",
	}
	verbs = []string{
		"Abandon", "Adapt", "Advertise", "Answer", "Anticipate", "Appreciate",
		"Approach", "Argue", "Ask", "Bite", "Blossom", "Blush", "Breathe", "Breed",
		"Bribe", "Burn", "Calculate", "Clean", "Code", "Communicate", "Compute",
		"Confess", "Confiscate", "Conjugate", "Conjure", "Consume", "Contemplate",
		"Crawl", "Dance", "Delegate", "Devour", "Develop", "Differ", "Discuss",
		"Dissolve", "Drink", "Eat", "Elaborate", "Emancipate", "Estimate", "Expire",
		"Extinguish", "Extract", "Facilitate", "Fall", "Feed", "Finish", "Floss",
		"Fly", "Follow", "Fragment", "Freeze", "Gather", "Glow", "Grow", "Hex",
		"Hide", "Hug", "Hurry", "Improve", "Intersect", "Investigate", "Jinx",
		"Joke", "Jubilate", "Invent", "Laugh", "Manage", "Meet", "Merge", "Move",
		"Object", "Observe", "Offer", "Paint", "Participate", "Party", "Perform",
		"Plan", "Pursue", "Pierce", "Play", "Postpone", "Call", "Proclaim",
		"Question", "Read", "Reckon", "Rejoice", "Represent", "Resize", "Rhyme",
		"Scream", "Search", "Select", "Share", "Shoot", "Shout", "Signal", "Sing",
		"Skate", "Sleep", "Smile", "Talk", "Solve", "Spell", "Steer", "Stink",
		"Substitute", "Swim", "Taste", "Teach", "Terminate", "Think", "Type",
		"Unite", "Vanish", "Meet",
	}
	adverbs = []string{
		"Absently", "Accurately", "Accusingly", "Adorably", "Majestically", "Alone",
		"Always", "Amazingly", "Angrily", "Anxiously", "Anywhere", "Appallingly",
		"Apparently", "Articulately", "Astonishingly", "Badly", "Barely",
		"Beautifully", "Blindly", "Bravely", "Brightly", "Briskly", "Brutally",
		"Calmly", "Carefully", "Casually", "Cautiously", "Cleverly", "Constantly",
		"Correctly", "Crazily", "Curiously", "Cynically", "Daily", "Dangerously",
		"Deliberately", "Delicately", "Desperately", "Discreetly", "Eagerly",
		"Easily", "Urgently", "Evenly", "Everywhere", "Exactly", "Expectantly",
		"Extensively", "Ferociously", "Fiercely", "Finely", "Flatly", "Frequently",
		"Frighteningly", "Gently", "Gloriously", "Grimly", "Guiltily", "Happily",
		"Hard", "Hastily", "Heroically", "High", "Highly", "Hourly", "Humbly",
		"Hysterically", "Immensely", "Impartially", "Impolitely", "Indifferently",
		"Intensely", "Jealously", "Jovially", "Kindly", "Lazily", "Lightly",
		"Loudly", "Lovingly", "Loyally", "Magnificently", "Initially", "Merrily",
		"Mightily", "Miserably", "Mysteriously", "NOT", "Nervously", "Nicely",
		"Nowhere", "Objectively", "Obnoxiously", "Obsessively", "Obviously",
		"Often", "Painfully", "Patiently", "Playfully", "Politely", "Poorly",
		"Precisely", "Promptly", "Quickly", "Quietly", "Randomly", "Rapidly",
		"Rarely", "Recklessly", "Regularly", "Remorsefully", "Responsibly",
		"Rudely", "Ruthlessly", "Sadly", "Scornfully", "Seamlessly", "Seldom",
		"Selfishly", "Seriously", "Shakily", "Sharply", "Sideways", "Silently",
		"Sleepily", "Slightly", "Slowly", "Slyly", "Smoothly", "Softly", "Solemnly",
		"Steadily", "Sternly", "Strangely", "Strongly", "Stunningly", "Surely",
		"Tenderly", "Thoughtfully", "Tightly", "Uneasily", "Vanishingly",
		"Hungrily", "Warmly", "Weakly", "Wearily", "Weekly", "Weirdly", "Well",
		"Well", "Recently", "Wildly", "Wisely", "Wonderfully", "Yearly",
	}
	adjectives = []string{
		"Abominable", "Accurate", "Adorable", "All", "Alleged", "Ancient", "Angry",
		"Anxious", "Appalling", "Apparent", "Astonishing", "Attractive", "Awesome",
		"Baby", "Bad", "Beautiful", "Benign", "Big", "Bitter", "Blind", "Blue",
		"Bold", "Brave", "Bright", "Brisk", "Calm", "Camouflaged", "Casual",
		"Cautious", "Choppy", "Chosen", "Clever", "Cold", "Cool", "Crawly",
		"Crazy", "Creepy", "Cruel", "Curious", "Cynical", "Dangerous", "Dark",
		"Delicate", "Desperate", "Difficult", "Discreet", "Disguised", "Dizzy",
		"Early", "Eager", "Easy", "Edgy", "Electric", "Elegant", "Emancipated",
		"Enormous", "Euphoric", "Giant", "Fast", "Ferocious", "Fierce", "Fine",
		"Flawed", "Flying", "Foolish", "Foxy", "Freezing", "Funny", "Furious",
		"Gentle", "Glorious", "Golden", "Good", "Green", "Green", "Guilty",
		"Hairy", "Happy", "Hard", "Hasty", "Hazy", "Heroic", "Hostile", "Historical",
		"Humble", "Humongous", "Humorous", "Hysterical", "Idealistic", "Ignorant",
		"Immense", "Impartial", "Impolite", "Indifferent", "Infuriated",
		"Insightful", "Intense", "Interesting", "Intimidated", "Intriguing",
		"Jealous", "Jolly", "Jovial", "Jumpy", "Kind", "Laughing", "Lazy", "Liquid",
		"Lonely", "Longing", "Loud", "Loving", "Loyal", "Macabre", "Mad", "Magical",
		"Magnificent", "Vague", "Medieval", "Memorable", "Mere", "Merry",
		"Mighty", "Mischievous", "Miserable", "Modified", "Moody", "Most",
		"Mysterious", "Mystical", "Needy", "Nervous", "Nice", "Objective",
		"Obnoxious", "Obsessive", "Obvious", "Opinionated", "Orange", "Painful",
		"Passionate", "Perfect", "Pink", "Playful", "Talented", "Polite", "Moderate",
		"Popular", "Powerful", "Precise", "Preserved", "Pretty", "Purple", "Quick",
		"Quiet", "Random", "Rapid", "Rare", "Real", "Reassuring", "Reckless", "Red",
		"Regular", "Remorseful", "Responsible", "Rich", "Rude", "Ruthless", "Sad",
		"Scared", "Scary", "Scornful", "Screaming", "Selfish", "Serious", "Shady",
		"Shaky", "Sharp", "Shiny", "Shy", "Simple", "Sleepy", "Slow", "Sly",
		"Small", "Smart", "Smelly", "Smiling", "Smooth", "Smug", "Windy", "Soft",
		"Solemn", "Square", "Square", "Steady", "Strange", "Strong", "Stunning",
		"Subjective", "Successful", "Surly", "Sweet", "Tactful", "Tense",
		"Thoughtful", "Tight", "Tiny", "Tolerant", "Uneasy", "Unique", "Unseen",
		"Warm", "Weak", "Weird", "Two", "Wild", "Wise", "Witty", "Wonderful",
		"Worried", "Yellow", "Young", "Zealous", "Three", "Four", "Five", "Six",
		"Seven", "Eight", "Nine", "Ten",
	}
	countNoun       = len(nouns)
	countVerb       = len(verbs)
	countAdverb     = len(adverbs)
	countAdjectives = len(adjectives)
)

// RandomName will generate a new video name randomly.
func RandomName() string {
	var (
		adj  = adjectives[rand.Intn(countAdjectives)]
		noun = nouns[rand.Intn(countNoun)]
		verb = verbs[rand.Intn(countVerb)]
		adv  = adverbs[rand.Intn(countAdverb)]
	)
	return adj + noun + verb + adv
}
