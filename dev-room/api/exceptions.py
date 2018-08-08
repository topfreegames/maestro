class WrongStatus(Exception):
    def __init__(self, text="Wrong status update. Try one of 'ready' or 'occupied'"):
        self.text=text
        super().__init__(self)

    def __str__(self):
        return str(self.text)