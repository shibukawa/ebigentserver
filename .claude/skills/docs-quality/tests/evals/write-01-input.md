# Writing case

`run.ResumeIndex` was added so a relaunch adds to a corpus instead of writing
over it: the match index carries the seed, so a run that started counting at
zero again would both truncate the previous episodes and replay their seeds.

Document it where a reader would look.
