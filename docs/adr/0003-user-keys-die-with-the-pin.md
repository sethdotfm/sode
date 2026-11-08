# A user's API key ends with their PIN; an API client's key does not

A user's key acts as that user in full. Session tokens already end on a PIN change, rename or removal, so a key that outlived them would be a weaker-bound way to be that user, and anyone at a signed-in wall browser could mint one that survives a PIN change. So a user's key ends on the same events, and creating one asks for the user's own PIN. API client keys belong to no PIN, so they survive PIN changes and are the path for a kiosk that must outlive them.

Considered: keys that survive PIN changes, which was the first answer. We dropped it because changing a PIN is the way people cut off a lost device, and it has to work for keys too.
